package collector

import (
	"context"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/kavix/k8s-advisor/internal/models"
)

// K8sCollector polls Kubernetes Metrics Server and watches core/v1 Events.
type K8sCollector struct {
	clusterName   string
	clientset     kubernetes.Interface
	metricsClient metricsv.Interface
}

// NewK8sCollector initializes Kubernetes clients using in-cluster config or ~/.kube/config.
func NewK8sCollector(clusterName string, inCluster bool) (*K8sCollector, error) {
	var cfg *rest.Config
	var err error
	if inCluster {
		cfg, err = rest.InClusterConfig()
	} else {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	mc, err := metricsv.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &K8sCollector{
		clusterName:   clusterName,
		clientset:     cs,
		metricsClient: mc,
	}, nil
}

// CollectPodMetrics polls metrics.k8s.io/v1beta1 for current Pod CPU and memory usage.
func (c *K8sCollector) CollectPodMetrics(ctx context.Context) ([]models.K8sMetricPoint, error) {
	now := time.Now().UTC()
	podMetrics, err := c.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	points := make([]models.K8sMetricPoint, 0, len(podMetrics.Items))
	for _, pm := range podMetrics.Items {
		var cpuMillicores float64
		var memBytes int64
		for _, ctr := range pm.Containers {
			cpuMillicores += float64(ctr.Usage.Cpu().MilliValue())
			memBytes += ctr.Usage.Memory().Value()
		}
		points = append(points, models.K8sMetricPoint{
			Time:          now,
			ClusterName:   c.clusterName,
			Namespace:     pm.Namespace,
			ResourceKind:  "Pod",
			ResourceName:  pm.Name,
			CPUMillicores: cpuMillicores,
			MemoryBytes:   memBytes,
			Labels:        pm.Labels,
		})
	}
	return points, nil
}

// WatchEvents streams core/v1 Events and sends parsed records to out until ctx is cancelled.
func (c *K8sCollector) WatchEvents(ctx context.Context, out chan<- models.K8sEventRecord) error {
	var resourceVersion string
	for {
		watcher, err := c.clientset.CoreV1().Events("").Watch(ctx, metav1.ListOptions{
			ResourceVersion:     resourceVersion,
			AllowWatchBookmarks: true,
		})
		if err != nil {
			return err
		}

		if err := c.consumeWatch(ctx, watcher, &resourceVersion, out); err != nil {
			watcher.Stop()
			return err
		}
		watcher.Stop()
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (c *K8sCollector) consumeWatch(
	ctx context.Context,
	watcher watch.Interface,
	resourceVersion *string,
	out chan<- models.K8sEventRecord,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			ev, ok := evt.Object.(*corev1.Event)
			if !ok {
				continue
			}
			*resourceVersion = ev.ResourceVersion
			if evt.Type == watch.Bookmark {
				continue
			}

			evTime := ev.LastTimestamp.Time
			if evTime.IsZero() {
				evTime = time.Now().UTC()
			}
			evType := "Normal"
			if ev.Type == corev1.EventTypeWarning {
				evType = "Warning"
			}

			rec := models.K8sEventRecord{
				Time:            evTime,
				EventUID:        string(ev.UID),
				ClusterName:     c.clusterName,
				Namespace:       ev.Namespace,
				InvolvedKind:    ev.InvolvedObject.Kind,
				InvolvedName:    ev.InvolvedObject.Name,
				EventType:       evType,
				Reason:          ev.Reason,
				Message:         ev.Message,
				SourceComponent: ev.Source.Component,
				Count:           int(ev.Count),
			}
			select {
			case out <- rec:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
