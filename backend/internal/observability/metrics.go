package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	DatabaseOperationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "movierec_database_operations_total",
		Help: "Total number of semantic database operations.",
	}, []string{"database", "operation", "result"})
	DatabaseOperationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "movierec_database_operation_duration_seconds",
		Help:    "Semantic database operation duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"database", "operation", "result"})
	DatabaseAvailability = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "movierec_database_available",
		Help: "Whether a database is currently available.",
	}, []string{"database"})
	ModelRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "movierec_model_requests_total",
		Help: "Total number of inference model requests.",
	}, []string{"model_type", "model", "operation", "result"})
	ModelRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "movierec_model_request_duration_seconds",
		Help:    "Inference model request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"model_type", "model", "operation", "result"})
	ModelReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "movierec_model_ready",
		Help: "Whether an inference model is ready.",
	}, []string{"model_type", "model"})
	RecommendationRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "movierec_recommendation_requests_total",
		Help: "Total number of recommendation pipeline requests.",
	}, []string{"operation", "result"})
	RecommendationRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "movierec_recommendation_request_duration_seconds",
		Help:    "Recommendation pipeline request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "result"})
)

func init() {
	prometheus.MustRegister(
		DatabaseOperationsTotal,
		DatabaseOperationDuration,
		DatabaseAvailability,
		ModelRequestsTotal,
		ModelRequestDuration,
		ModelReady,
		RecommendationRequestsTotal,
		RecommendationRequestDuration,
	)
}

func ObserveDatabaseAvailability(database string, available bool) {
	value := 0.0
	if available {
		value = 1
	}
	DatabaseAvailability.WithLabelValues(database).Set(value)
}

func ObserveDatabaseOperation(database, operation, result string, started time.Time) {
	DatabaseOperationsTotal.WithLabelValues(database, operation, result).Inc()
	DatabaseOperationDuration.WithLabelValues(database, operation, result).Observe(time.Since(started).Seconds())
}

func ObserveModelReady(modelType, model string, ready bool) {
	value := 0.0
	if ready {
		value = 1
	}
	ModelReady.WithLabelValues(modelType, model).Set(value)
}

func ObserveModelRequest(modelType, model, operation, result string, started time.Time) {
	ModelRequestsTotal.WithLabelValues(modelType, model, operation, result).Inc()
	ModelRequestDuration.WithLabelValues(modelType, model, operation, result).Observe(time.Since(started).Seconds())
}

func ObserveRecommendationRequest(operation, result string, started time.Time) {
	RecommendationRequestsTotal.WithLabelValues(operation, result).Inc()
	RecommendationRequestDuration.WithLabelValues(operation, result).Observe(time.Since(started).Seconds())
}
