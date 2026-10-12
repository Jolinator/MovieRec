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
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
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

func RequestMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseStatusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		labels := prometheus.Labels{
			"method": r.Method,
			"route":  route,
			"status": strconv.Itoa(status),
		}
		httpRequestsTotal.With(labels).Inc()
		httpRequestDuration.With(labels).Observe(time.Since(started).Seconds())
	})
}
