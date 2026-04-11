package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	appUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "github_subscription",
		Name:      "app_up",
		Help:      "Whether the service is up.",
	})
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "github_subscription",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "github_subscription",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	scannerCyclesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "github_subscription",
			Name:      "scanner_cycles_total",
			Help:      "Total number of scanner cycles by result.",
		},
		[]string{"result"},
	)
	githubRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "github_subscription",
			Name:      "github_requests_total",
			Help:      "Total number of GitHub API requests by endpoint and status.",
		},
		[]string{"endpoint", "status"},
	)
	githubRateLimitTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "github_subscription",
			Name:      "github_rate_limit_total",
			Help:      "Total number of GitHub API rate limit responses.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		appUp,
		httpRequestsTotal,
		httpRequestDuration,
		scannerCyclesTotal,
		githubRequestsTotal,
		githubRateLimitTotal,
	)
}

func SetAppUp() {
	appUp.Set(1)
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func TrackHTTPRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		status := strconv.Itoa(recorder.statusCode)
		path := routePattern(r)
		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path, status).Observe(time.Since(start).Seconds())
	})
}

func RecordScannerCycle(result string) {
	scannerCyclesTotal.WithLabelValues(result).Inc()
}

func RecordGitHubRequest(endpoint string, statusCode int) {
	githubRequestsTotal.WithLabelValues(endpoint, strconv.Itoa(statusCode)).Inc()
}

func RecordGitHubRateLimit() {
	githubRateLimitTotal.Inc()
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func routePattern(r *http.Request) string {
	if pattern := r.Pattern; pattern != "" {
		return pattern
	}

	return r.URL.Path
}
