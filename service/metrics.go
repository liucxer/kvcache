package service

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsRegistered sync.Once
)

// Metrics 监控指标
type Metrics struct {
	// 操作计数
	Sets          prometheus.Counter
	Gets          prometheus.Counter
	Deletes       prometheus.Counter
	Scans         prometheus.Counter
	MSets         prometheus.Counter
	MGets         prometheus.Counter
	MDeletes      prometheus.Counter
	ConfigUpdates prometheus.Counter
	HealthChecks  prometheus.Counter

	// 错误计数
	SetErrors     *prometheus.CounterVec
	GetErrors     *prometheus.CounterVec
	DeleteErrors  *prometheus.CounterVec
	ScanErrors    *prometheus.CounterVec
	MSetErrors    *prometheus.CounterVec
	MGetErrors    *prometheus.CounterVec
	MDeleteErrors *prometheus.CounterVec

	// 延迟指标
	SetLatency         *prometheus.HistogramVec
	GetLatency         *prometheus.HistogramVec
	DeleteLatency      *prometheus.HistogramVec
	ScanLatency        *prometheus.HistogramVec
	MSetLatency        *prometheus.HistogramVec
	MGetLatency        *prometheus.HistogramVec
	MDeleteLatency     *prometheus.HistogramVec
	HealthCheckLatency prometheus.Histogram

	// 状态指标
	Keys        prometheus.Gauge
	DiskUsage   prometheus.Gauge
	MemoryUsage prometheus.Gauge
}

// NewMetrics 创建新的监控指标实例
func NewMetrics() *Metrics {
	metrics := &Metrics{
		// 操作计数
		Sets: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "sets_total",
			Help:      "Total number of set operations",
		}),
		Gets: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "gets_total",
			Help:      "Total number of get operations",
		}),
		Deletes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "deletes_total",
			Help:      "Total number of delete operations",
		}),
		Scans: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "scans_total",
			Help:      "Total number of scan operations",
		}),
		MSets: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "msets_total",
			Help:      "Total number of multi-set operations",
		}),
		MGets: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mgets_total",
			Help:      "Total number of multi-get operations",
		}),
		MDeletes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mdeletes_total",
			Help:      "Total number of multi-delete operations",
		}),
		ConfigUpdates: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "config",
			Name:      "updates_total",
			Help:      "Total number of configuration updates",
		}),
		HealthChecks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "health",
			Name:      "checks_total",
			Help:      "Total number of health checks",
		}),

		// 错误计数
		SetErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "set_errors_total",
			Help:      "Total number of set operation errors",
		}, []string{"error"}),
		GetErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "get_errors_total",
			Help:      "Total number of get operation errors",
		}, []string{"error"}),
		DeleteErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "delete_errors_total",
			Help:      "Total number of delete operation errors",
		}, []string{"error"}),
		ScanErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "scan_errors_total",
			Help:      "Total number of scan operation errors",
		}, []string{"error"}),
		MSetErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mset_errors_total",
			Help:      "Total number of multi-set operation errors",
		}, []string{"error"}),
		MGetErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mget_errors_total",
			Help:      "Total number of multi-get operation errors",
		}, []string{"error"}),
		MDeleteErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mdelete_errors_total",
			Help:      "Total number of multi-delete operation errors",
		}, []string{"error"}),

		// 延迟指标
		SetLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "set_latency_seconds",
			Help:      "Set operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		GetLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "get_latency_seconds",
			Help:      "Get operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		DeleteLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "delete_latency_seconds",
			Help:      "Delete operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		ScanLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "scan_latency_seconds",
			Help:      "Scan operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		MSetLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mset_latency_seconds",
			Help:      "Multi-set operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		MGetLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mget_latency_seconds",
			Help:      "Multi-get operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		MDeleteLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "mdelete_latency_seconds",
			Help:      "Multi-delete operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"type"}),
		HealthCheckLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cachefs",
			Subsystem: "health",
			Name:      "check_latency_seconds",
			Help:      "Health check latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}),

		// 状态指标
		Keys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "cachefs",
			Subsystem: "kv",
			Name:      "keys_current",
			Help:      "Current number of keys in storage",
		}),
		DiskUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "cachefs",
			Subsystem: "storage",
			Name:      "disk_usage_bytes",
			Help:      "Current disk usage in bytes",
		}),
		MemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "cachefs",
			Subsystem: "storage",
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		}),
	}

	// 注册指标（仅注册一次）
	metricsRegistered.Do(func() {
		prometheus.MustRegister(
			metrics.Sets,
			metrics.Gets,
			metrics.Deletes,
			metrics.Scans,
			metrics.MSets,
			metrics.MGets,
			metrics.MDeletes,
			metrics.ConfigUpdates,
			metrics.HealthChecks,
			metrics.SetErrors,
			metrics.GetErrors,
			metrics.DeleteErrors,
			metrics.ScanErrors,
			metrics.MSetErrors,
			metrics.MGetErrors,
			metrics.MDeleteErrors,
			metrics.SetLatency,
			metrics.GetLatency,
			metrics.DeleteLatency,
			metrics.ScanLatency,
			metrics.MSetLatency,
			metrics.MGetLatency,
			metrics.MDeleteLatency,
			metrics.HealthCheckLatency,
			metrics.Keys,
			metrics.DiskUsage,
			metrics.MemoryUsage,
		)
	})

	return metrics
}
