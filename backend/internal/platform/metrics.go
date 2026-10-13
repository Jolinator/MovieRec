package platform

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests handled by the API.",
	}, []string{"method", "route", "status"})
	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests handled by the API.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	applicationRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "movierec_http_requests_total",
		Help: "Total number of application HTTP requests.",
	}, []string{"service", "method", "route", "status_class"})
	applicationRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "movierec_http_request_duration_seconds",
		Help:    "Application HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "route", "status_class"})
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		applicationRequestsTotal,
		applicationRequestDuration,
	)
}

type responseStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseStatusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseStatusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// RequestMetricsMiddleware records public application requests. The optional service argument
// keeps the middleware compatible with standalone handlers and gives deployed services a stable label.
func RequestMetricsMiddleware(next http.Handler, service ...string) http.Handler {
	serviceName := "unknown"
	if len(service) > 0 && service[0] != "" {
		serviceName = service[0]
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseStatusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		if r.URL.Path == "/metrics" {
			return
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		route := r.Pattern
		if route == "" {
			route = requestRoute(r)
		}
		statusText := strconv.Itoa(status)
		statusClass := strconv.Itoa(status/100) + "xx"
		httpRequestsTotal.WithLabelValues(r.Method, route, statusText).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route, statusText).Observe(time.Since(started).Seconds())
		applicationRequestsTotal.WithLabelValues(serviceName, r.Method, route, statusClass).Inc()
		applicationRequestDuration.WithLabelValues(serviceName, r.Method, route, statusClass).Observe(time.Since(started).Seconds())
	})
}

func requestRoute(r *http.Request) string {
	if r.Method == http.MethodOptions {
		return "cors_preflight"
	}
	return "unmatched"
}
