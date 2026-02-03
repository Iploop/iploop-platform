package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Request metrics
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iploop_requests_total",
			Help: "Total number of proxy requests",
		},
		[]string{"protocol", "country", "status"},
	)

	requestsSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iploop_requests_success_total",
			Help: "Total number of successful proxy requests",
		},
		[]string{"protocol", "country"},
	)

	requestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iploop_requests_failed_total",
			Help: "Total number of failed proxy requests",
		},
		[]string{"protocol", "country", "error_type"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "iploop_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"protocol", "country"},
	)

	// Bandwidth metrics
	bytesTransferred = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iploop_bytes_transferred_total",
			Help: "Total bytes transferred",
		},
		[]string{"direction", "country"},
	)

	bytesUploaded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iploop_bytes_uploaded_total",
			Help: "Total bytes uploaded",
		},
	)

	bytesDownloaded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iploop_bytes_downloaded_total",
			Help: "Total bytes downloaded",
		},
	)

	// Node metrics
	nodesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_nodes_total",
			Help: "Total number of registered nodes",
		},
	)

	nodesAvailable = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_nodes_available",
			Help: "Number of available nodes",
		},
	)

	nodesBusy = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_nodes_busy",
			Help: "Number of busy nodes",
		},
	)

	nodesOffline = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_nodes_offline",
			Help: "Number of offline nodes",
		},
	)

	nodesByCountry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iploop_nodes_by_country",
			Help: "Number of nodes by country",
		},
		[]string{"country"},
	)

	// Customer metrics
	customersActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_customers_active",
			Help: "Number of active customers",
		},
	)

	// Connection metrics
	connectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_connections_active",
			Help: "Number of active connections",
		},
	)

	wsConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iploop_ws_connections_active",
			Help: "Number of active WebSocket node connections",
		},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		requestsTotal,
		requestsSuccess,
		requestsFailed,
		requestDuration,
		bytesTransferred,
		bytesUploaded,
		bytesDownloaded,
		nodesTotal,
		nodesAvailable,
		nodesBusy,
		nodesOffline,
		nodesByCountry,
		customersActive,
		connectionsActive,
		wsConnectionsActive,
	)
}

// PrometheusCollector provides methods to record metrics
type PrometheusCollector struct{}

// NewPrometheusCollector creates a new Prometheus collector
func NewPrometheusCollector() *PrometheusCollector {
	return &PrometheusCollector{}
}

// Handler returns the Prometheus HTTP handler
func (c *PrometheusCollector) Handler() http.Handler {
	return promhttp.Handler()
}

// RecordRequest records a proxy request
func (c *PrometheusCollector) RecordRequest(protocol, country, status string) {
	requestsTotal.WithLabelValues(protocol, country, status).Inc()
}

// RecordSuccess records a successful request
func (c *PrometheusCollector) RecordSuccess(protocol, country string, duration time.Duration) {
	requestsSuccess.WithLabelValues(protocol, country).Inc()
	requestDuration.WithLabelValues(protocol, country).Observe(duration.Seconds())
}

// RecordFailure records a failed request
func (c *PrometheusCollector) RecordFailure(protocol, country, errorType string) {
	requestsFailed.WithLabelValues(protocol, country, errorType).Inc()
}

// RecordBytes records bytes transferred
func (c *PrometheusCollector) RecordBytes(uploaded, downloaded int64, country string) {
	bytesTransferred.WithLabelValues("upload", country).Add(float64(uploaded))
	bytesTransferred.WithLabelValues("download", country).Add(float64(downloaded))
	bytesUploaded.Add(float64(uploaded))
	bytesDownloaded.Add(float64(downloaded))
}

// SetNodeCounts sets the current node counts
func (c *PrometheusCollector) SetNodeCounts(total, available, busy, offline int) {
	nodesTotal.Set(float64(total))
	nodesAvailable.Set(float64(available))
	nodesBusy.Set(float64(busy))
	nodesOffline.Set(float64(offline))
}

// SetNodeCountsByCountry sets node counts by country
func (c *PrometheusCollector) SetNodeCountsByCountry(countries map[string]int) {
	for country, count := range countries {
		nodesByCountry.WithLabelValues(country).Set(float64(count))
	}
}

// SetActiveCustomers sets the number of active customers
func (c *PrometheusCollector) SetActiveCustomers(count int) {
	customersActive.Set(float64(count))
}

// SetActiveConnections sets the number of active connections
func (c *PrometheusCollector) SetActiveConnections(count int) {
	connectionsActive.Set(float64(count))
}

// SetWSConnections sets the number of active WebSocket connections
func (c *PrometheusCollector) SetWSConnections(count int) {
	wsConnectionsActive.Set(float64(count))
}

// IncrementConnections increments active connections
func (c *PrometheusCollector) IncrementConnections() {
	connectionsActive.Inc()
}

// DecrementConnections decrements active connections
func (c *PrometheusCollector) DecrementConnections() {
	connectionsActive.Dec()
}
