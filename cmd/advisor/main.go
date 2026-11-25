package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/kavix/k8s-advisor/internal/advisor"
	"github.com/kavix/k8s-advisor/internal/api"
	"github.com/kavix/k8s-advisor/internal/classifier"
	"github.com/kavix/k8s-advisor/internal/config"
	"github.com/kavix/k8s-advisor/internal/storage"
	"github.com/kavix/k8s-advisor/internal/telemetry"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownOTel, err := telemetry.Init(ctx, cfg.OTelServiceName, cfg.OTelExporterOTLPEndpt, cfg.OTelEnabled)
	if err != nil {
		log.Printf("warning: telemetry init failed: %v", err)
	} else {
		defer func() {
			sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownOTel(sCtx)
		}()
	}

	store, err := storage.NewTimescaleRepository(ctx, cfg.TimescaleDSN)
	if err != nil {
		log.Fatalf("failed to connect to TimescaleDB: %v", err)
	}
	defer store.Close()

	cls := classifier.NewJevClassifier(cfg.TypeSafeAPIKey, cfg.TypeSafeBaseURL, cfg.JevEnabled)
	engine := advisor.NewEngine(store, cls)
	srv := api.NewServer(store, engine)

	httpSrv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("k8s-advisor listening on %s", cfg.APIAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down k8s-advisor...")
	sCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(sCtx)
}
