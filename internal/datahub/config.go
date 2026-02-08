package datahub

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	APIKey          string
	CustomerContext string
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".terminat", "config.toml"), nil
}

// LoadConfig reads the [datahub] section from ~/.terminat/config.toml
func LoadConfig() Config {
	path, err := configPath()
	if err != nil {
		return Config{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	return parseConfig(string(data))
}

func parseConfig(content string) Config {
	var cfg Config
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "[datahub]" {
			inSection = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		switch key {
		case "api_key":
			cfg.APIKey = val
		case "customer_context":
			cfg.CustomerContext = val
		}
	}
	return cfg
}

// SaveConfig writes the [datahub] section to ~/.terminat/config.toml, preserving other sections.
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	existing, _ := os.ReadFile(path)
	content := string(existing)

	section := "[datahub]\napi_key = \"" + cfg.APIKey + "\"\ncustomer_context = \"" + cfg.CustomerContext + "\"\n"

	// Replace existing [datahub] section or append
	if idx := strings.Index(content, "[datahub]"); idx >= 0 {
		end := strings.Index(content[idx+1:], "\n[")
		if end < 0 {
			content = content[:idx] + section
		} else {
			content = content[:idx] + section + content[idx+1+end+1:]
		}
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += section
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// ResolveAPIKey returns the API key from flag > env > config.
func ResolveAPIKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("DOIT_DATAHUB_API_KEY"); v != "" {
		return v
	}
	return LoadConfig().APIKey
}

// ResolveCustomerContext returns the customer context from flag > env > config.
func ResolveCustomerContext(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("DOIT_CUSTOMER_CONTEXT"); v != "" {
		return v
	}
	return LoadConfig().CustomerContext
}
