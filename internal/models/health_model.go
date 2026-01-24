package models

var TYPE_HEALTH = "HEALTH"

type HealthResponse struct {
	Description  string                 `json:"description"`
	Dependencies []HealthDetailResponse `json:"dependencies"`
}

type (
	HealthDetailResponse struct {
		Type        string `json:"type"`
		Component   string `json:"component"`
		Status      string `json:"status"`
		Description string `json:"description,omitempty"`
	}
)
