package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the Oak configuration loaded from oak.yaml
type Config struct {
	// Packages is a list of package paths to scan for //go:generate oak directives
	Packages []string `yaml:"packages"`
	
	// RedactKeys is a list of field names to automatically redact (case-insensitive)
	RedactKeys []string `yaml:"redactKeys"`
	
	// RedactMessage is the message to use for redacted fields
	RedactMessage string `yaml:"redactMessage"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Packages:      []string{"."},
		RedactKeys:    []string{},
		RedactMessage: "[REDACTED]",
	}
}

// LoadConfig loads the oak.yaml configuration file from the current directory
// or parent directories, searching upward until found or reaching the root
func LoadConfig() (*Config, error) {
	configPath, err := findConfigFile()
	if err != nil {
		return nil, fmt.Errorf("oak.yaml configuration file not found in current directory or parent directories")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate and normalize the configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", configPath, err)
	}

	return config, nil
}

// LoadConfigFromPath loads the oak.yaml configuration file from a specific path
func LoadConfigFromPath(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate and normalize the configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", configPath, err)
	}

	return config, nil
}

// findConfigFile searches for oak.yaml starting from the current directory
// and moving up the directory tree until found or reaching the root
func findConfigFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, "oak.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("oak.yaml not found")
}

// validate checks the configuration for common errors and normalizes values
func (c *Config) validate() error {
	// Normalize redact keys to lowercase for case-insensitive matching
	for i, key := range c.RedactKeys {
		c.RedactKeys[i] = strings.ToLower(key)
	}

	// Ensure redact message is not empty
	if c.RedactMessage == "" {
		c.RedactMessage = "[REDACTED]"
	}

	// Validate package paths exist (basic validation)
	for _, pkg := range c.Packages {
		if pkg == "" {
			return fmt.Errorf("empty package path in packages list")
		}
		// Convert relative paths to absolute for validation
		if !filepath.IsAbs(pkg) {
			absPath, err := filepath.Abs(pkg)
			if err != nil {
				return fmt.Errorf("invalid package path %s: %w", pkg, err)
			}
			// Check if the path exists
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return fmt.Errorf("package path does not exist: %s", pkg)
			}
		}
	}

	return nil
}

// ShouldRedactField checks if a field name should be redacted based on the configuration
func (c *Config) ShouldRedactField(fieldName string) bool {
	fieldLower := strings.ToLower(fieldName)
	for _, redactKey := range c.RedactKeys {
		if fieldLower == redactKey {
			return true
		}
	}
	return false
}

// GetPackages returns the list of packages to process
func (c *Config) GetPackages() []string {
	if len(c.Packages) == 0 {
		return []string{"."}
	}
	return c.Packages
}
