package common

// Contract of the service-to-service API served on each service's internal listener.
const (
	InternalHealthPath = "/internal/health"

	InternalRecommendationsUsersPath = "/internal/recommendations/users/"
	InternalEnsureFreshAction        = "ensure-fresh"
	InternalRequestRefreshAction     = "request-refresh"
)

// ServiceHealth is what each service reports about itself; the gateway merges these into HealthResponse.
type ServiceHealth struct {
	DatabaseAvailable bool `json:"databaseAvailable"`
	Ready             bool `json:"ready"`
}
