package commands

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

// MetricsData holds enhanced metrics information for command execution
type MetricsData struct {
	FlagsUsed    []string `json:"flags_used,omitempty"`
	OS           string   `json:"os,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	IsCI         bool     `json:"is_ci,omitempty"`
	CISystem     string   `json:"ci_system,omitempty"`
	IsContainer  bool     `json:"is_container,omitempty"`
}

// metricsCollector provides thread-safe collection and storage of command metrics
type metricsCollector struct {
	mu   sync.RWMutex
	data map[string]*MetricsData
}

var globalMetricsCollector = &metricsCollector{
	data: make(map[string]*MetricsData),
}

// CollectMetrics stores enhanced metrics data for a command execution.
// Collects system information, CI environment details, and container detection.
func CollectMetrics(commandName string, flags []string) {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()

	ciSystem := detectCISystem()
	isCI := ciSystem != ""

	if ciSystem == "" {
		ciSystem = "unknown"
	}

	metricsData := &MetricsData{
		FlagsUsed:    flags,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		IsCI:         isCI,
		CISystem:     ciSystem,
		IsContainer:  isRunningInContainer(),
	}

	globalMetricsCollector.data[commandName] = metricsData
}

// GetCollectedMetrics retrieves collected metrics for a command.
// Returns a copy of the metrics data without clearing the original.
func GetCollectedMetrics(commandName string) *MetricsData {
	globalMetricsCollector.mu.RLock()
	metrics, exists := globalMetricsCollector.data[commandName]
	globalMetricsCollector.mu.RUnlock()

	if !exists {
		return nil
	}

	return &MetricsData{
		FlagsUsed:    append([]string(nil), metrics.FlagsUsed...),
		OS:           metrics.OS,
		Architecture: metrics.Architecture,
		IsCI:         metrics.IsCI,
		CISystem:     metrics.CISystem,
		IsContainer:  metrics.IsContainer,
	}
}

// detectCISystem identifies the CI environment and returns the system name
func detectCISystem() string {
	ciEnvVars := map[string]string{
		"JENKINS_URL":            "jenkins",
		"TRAVIS":                 "travis",
		"CIRCLECI":               "circleci",
		"GITHUB_ACTIONS":         "github_actions",
		"GITLAB_CI":              "gitlab",
		"BUILDKITE":              "buildkite",
		"BAMBOO_BUILD_KEY":       "bamboo",
		"TF_BUILD":               "azure_devops",
		"TEAMCITY_VERSION":       "teamcity",
		"DRONE":                  "drone",
		"BITBUCKET_BUILD_NUMBER": "bitbucket",
		"CODEBUILD_BUILD_ID":     "aws_codebuild",
	}

	for envVar, system := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return system
		}
	}

	genericCIVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"BUILD_ID",
		"BUILD_NUMBER",
	}

	for _, envVar := range genericCIVars {
		if os.Getenv(envVar) != "" {
			return "unknown"
		}
	}

	return ""
}

// isRunningInContainer detects if the CLI is running inside a container
func isRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	if os.Getenv("container") != "" {
		return true
	}

	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "lxc") {
			return true
		}
	}

	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") {
			return true
		}
	}

	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "overlay") {
			return true
		}
	}

	return false
}

// ClearCollectedMetrics removes metrics data for a specific command
func ClearCollectedMetrics(commandName string) {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()
	delete(globalMetricsCollector.data, commandName)
}

// ClearAllMetrics clears all stored metrics
func ClearAllMetrics() {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()
	globalMetricsCollector.data = make(map[string]*MetricsData)
}

var contextFlags []string
var contextFlagsMutex sync.Mutex

// SetContextFlags stores flags for the current command execution
func SetContextFlags(flags []string) {
	contextFlagsMutex.Lock()
	defer contextFlagsMutex.Unlock()
	contextFlags = append([]string(nil), flags...)
}

// GetContextFlags retrieves and clears the stored flags
func GetContextFlags() []string {
	contextFlagsMutex.Lock()
	defer contextFlagsMutex.Unlock()
	flags := contextFlags
	contextFlags = nil
	return flags
}

// TransferMetrics moves metrics from one command name to another.
// Useful when collecting metrics with a temporary key that needs to be moved to the actual command name.
func TransferMetrics(fromCommandName, toCommandName string) bool {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()

	if metricsData, exists := globalMetricsCollector.data[fromCommandName]; exists {
		globalMetricsCollector.data[toCommandName] = metricsData
		delete(globalMetricsCollector.data, fromCommandName)
		return true
	}
	return false
}
