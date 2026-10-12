package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func counterValue(t *testing.T, method, route, status string) float64 {
	t.Helper()
	var metric dto.Metric
	if err := httpRequestsTotal.WithLabelValues(method, route, status).Write(&metric); err != nil {
		t.Fatal(err)
	}
	return metric.GetCounter().GetValue()
}

func TestResponseStatusRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseStatusRecorder{ResponseWriter: rec}
	w.WriteHeader(http.StatusTeapot)
	w.WriteHeader(http.StatusOK) // ignored
	if w.status != http.StatusTeapot || rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d / %d", w.status, rec.Code)
	}

	rec = httptest.NewRecorder()
	w = &responseStatusRecorder{ResponseWriter: rec}
	if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if w.status != http.StatusOK {
		t.Fatalf("implicit status = %d", w.status)
	}
}

func TestRequestMetricsMiddlewareCountsRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics-test", RequestMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})))

	before := counterValue(t, "GET", "GET /metrics-test", "202")
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/metrics-test", nil))
	if got := counterValue(t, "GET", "GET /metrics-test", "202") - before; got != 1 {
		t.Fatalf("counter delta = %v, want 1", got)
	}
}

func TestRequestMetricsMiddlewareUnmatchedAndDefaultStatus(t *testing.T) {
	handler := RequestMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	before := counterValue(t, "GET", "unmatched", "200")
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	if got := counterValue(t, "GET", "unmatched", "200") - before; got != 1 {
		t.Fatalf("counter delta = %v, want 1", got)
	}
}
