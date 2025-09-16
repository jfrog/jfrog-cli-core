package commands

import (
	"os"
	"runtime"
	"strings"
	"sync"

	metrics "github.com/jfrog/jfrog-cli-core/v2/utils/metrics"
)

// MetricsData is shared from utils/metrics to avoid import cycles.
type MetricsData = metrics.MetricsData

// metricsCollector provides thread-safe collection and storage of command metrics
type metricsCollector struct {
	mu          sync.RWMutex
	metricsData map[string]*MetricsData
}

var contextFlags []string
var globalMetricsCollector = &metricsCollector{
	metricsData: make(map[string]*MetricsData),
}

// CollectMetrics stores enhanced metrics information for a command execution.
// Collects system information, CI environment details, and container detection.
func CollectMetrics(commandName string, flags []string) {
	globalMetricsCollector.mu.Lock()
	defer globalMetricsCollector.mu.Unlock()

	ciSystem := detectCISystem()
	isCI := ciSystem != ""

	metricsData := &MetricsData{
		Flags:        flags,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		IsCI:         isCI,
		CISystem: func() string {
			if isCI {
				return ciSystem
			}
			return ""
		}(),
		IsContainer: isRunningInContainer(),
	}

	globalMetricsCollector.metricsData[commandName] = metricsData
}

// GetCollectedMetrics retrieves collected metrics for a command.
// Returns a copy of the metrics data without clearing the original.
func GetCollectedMetrics(commandName string) *MetricsData {
	globalMetricsCollector.mu.RLock()
	metrics, exists := globalMetricsCollector.metricsData[commandName]
	globalMetricsCollector.mu.RUnlock()

	if !exists {
		return nil
	}

	return &MetricsData{
		Flags:        append([]string(nil), metrics.Flags...),
		Platform:     metrics.Platform,
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

// SetContextFlags stores flags for the current command execution
func SetContextFlags(flags []string) {
	contextFlags = append([]string(nil), flags...)
}

// GetContextFlags retrieves and clears the stored flags
func GetContextFlags() []string {
	flags := contextFlags
	contextFlags = nil
	return flags
}
