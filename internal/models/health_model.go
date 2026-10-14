package models

var TYPE_HEALTH = "HEALTH"

type HealthResponse struct {
	Description string `json:"description"`
	// Healthy is false when any dependency is down; the handler answers 503.
	Healthy      bool                   `json:"-"`
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
