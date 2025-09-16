package metrics

// MetricsData holds enhanced metrics information for command execution.
// Shared between commands and visibility packages to avoid import cycles.
type MetricsData struct {
	Flags        []string `json:"flags,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	IsCI         bool     `json:"is_ci,omitempty"`
	CISystem     string   `json:"ci_system,omitempty"`
	IsContainer  bool     `json:"is_container,omitempty"`
}
