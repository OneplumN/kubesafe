package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	HTTPAddr       string
	KubeconfigPath string
	Mode           string
	Update         UpdateConfig
}

type UpdateConfig struct {
	Enabled          bool
	AllowPrereleases bool
	Repository       string
	AllowedSubjects  []string
	GitHubToken      string
	TargetPath       string
}

func Load() Config {
	return Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		KubeconfigPath: kubeconfigPath(),
		Mode:           runtimeMode(),
		Update: UpdateConfig{
			Enabled:          getEnv("KUBESAFE_UPDATE_ENABLED", "") == "true",
			AllowPrereleases: getEnv("KUBESAFE_UPDATE_ALLOW_PRERELEASES", "") == "true",
			Repository:       getEnv("KUBESAFE_UPDATE_REPOSITORY", "OneplumN/kubesafe"),
			AllowedSubjects:  splitCSVEnv("KUBESAFE_UPDATE_ALLOWED_SUBJECTS"),
			GitHubToken:      getEnv("KUBESAFE_UPDATE_GITHUB_TOKEN", ""),
			TargetPath:       getEnv("KUBESAFE_UPDATE_TARGET_PATH", ""),
		},
	}
}

func (c Config) SimulationMode() bool {
	return c.Mode == "simulation"
}

func runtimeMode() string {
	value := strings.ToLower(getEnv("KUBESAFE_MODE", "real"))
	if value == "simulation" {
		return value
	}

	return "real"
}

func kubeconfigPath() string {
	if value := strings.TrimSpace(os.Getenv("KUBESAFE_KUBECONFIG")); value != "" {
		return value
	}

	if value := strings.TrimSpace(os.Getenv("KUBECONFIG")); value != "" {
		parts := strings.Split(value, string(os.PathListSeparator))
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".kube", "config")
}

func getEnv(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func splitCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
