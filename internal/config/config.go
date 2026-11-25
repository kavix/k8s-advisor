package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration for k8s-advisor loaded from environment variables.
type Config struct {
	K8sInCluster           bool
	K8sClusterName         string
	MetricsPollInterval    time.Duration
	TimescaleDSN           string
	TypeSafeAPIKey         string
	TypeSafeBaseURL        string
	JevEnabled             bool
	OTelServiceName        string
	OTelExporterOTLPEndpt  string
	OTelEnabled            bool
	APIAddr                string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		K8sInCluster:          getBool("K8S_IN_CLUSTER", false),
		K8sClusterName:        getEnv("K8S_CLUSTER_NAME", "default-cluster"),
		MetricsPollInterval:   time.Duration(getInt("K8S_METRICS_POLL_INTERVAL_SECONDS", 15)) * time.Second,
		TimescaleDSN:          getEnv("TIMESCALE_DSN", "postgresql://tsdbadmin:password@localhost:5432/tsdb?sslmode=require"),
		TypeSafeAPIKey:        getEnv("TYPESAFE_API_KEY", ""),
		TypeSafeBaseURL:       getEnv("TYPESAFE_BASE_URL", "https://api.typesafe.ai"),
		JevEnabled:            getBool("JEV_ENABLED", true),
		OTelServiceName:       getEnv("OTEL_SERVICE_NAME", "k8s-advisor"),
		OTelExporterOTLPEndpt: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		OTelEnabled:           getBool("OTEL_ENABLED", true),
		APIAddr:               ":" + getEnv("API_PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
