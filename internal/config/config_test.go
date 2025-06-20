package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if len(config.Packages) != 1 || config.Packages[0] != "." {
		t.Errorf("Expected default packages to be ['.'], got %v", config.Packages)
	}
	
	if len(config.RedactKeys) != 0 {
		t.Errorf("Expected default redact keys to be empty, got %v", config.RedactKeys)
	}
	
	if config.RedactMessage != "[REDACTED]" {
		t.Errorf("Expected default redact message to be '[REDACTED]', got %s", config.RedactMessage)
	}
}

func TestShouldRedactField(t *testing.T) {
	config := &Config{
		RedactKeys: []string{"password", "secret", "api_key"},
	}
	
	testCases := []struct {
		fieldName string
		expected  bool
	}{
		{"password", true},
		{"Password", true},
		{"PASSWORD", true},
		{"secret", true},
		{"Secret", true},
		{"api_key", true},
		{"API_KEY", true},
		{"username", false},
		{"email", false},
		{"id", false},
	}
	
	for _, tc := range testCases {
		result := config.ShouldRedactField(tc.fieldName)
		if result != tc.expected {
			t.Errorf("ShouldRedactField(%s) = %v, expected %v", tc.fieldName, result, tc.expected)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	// Test empty redact message gets default
	config := &Config{
		Packages:      []string{"."},
		RedactKeys:    []string{"Password", "SECRET"},
		RedactMessage: "",
	}
	
	err := config.validate()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}
	
	if config.RedactMessage != "[REDACTED]" {
		t.Errorf("Expected empty redact message to be set to default, got %s", config.RedactMessage)
	}
	
	// Test redact keys are normalized to lowercase
	expectedKeys := []string{"password", "secret"}
	for i, key := range config.RedactKeys {
		if key != expectedKeys[i] {
			t.Errorf("Expected redact key %d to be %s, got %s", i, expectedKeys[i], key)
		}
	}
}

func TestLoadConfigFromPath(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "oak.yaml")
	
	configContent := `packages:
  - ./test1
  - ./test2
redactKeys:
  - password
  - secret
redactMessage: "[HIDDEN]"`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	// Create test directories so validation passes
	os.MkdirAll(filepath.Join(tempDir, "test1"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "test2"), 0755)
	
	// Change to temp directory for relative path validation
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)
	
	config, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	expectedPackages := []string{"./test1", "./test2"}
	if len(config.Packages) != len(expectedPackages) {
		t.Errorf("Expected %d packages, got %d", len(expectedPackages), len(config.Packages))
	}
	
	for i, pkg := range config.Packages {
		if pkg != expectedPackages[i] {
			t.Errorf("Expected package %d to be %s, got %s", i, expectedPackages[i], pkg)
		}
	}
	
	expectedRedactKeys := []string{"password", "secret"}
	if len(config.RedactKeys) != len(expectedRedactKeys) {
		t.Errorf("Expected %d redact keys, got %d", len(expectedRedactKeys), len(config.RedactKeys))
	}
	
	if config.RedactMessage != "[HIDDEN]" {
		t.Errorf("Expected redact message to be '[HIDDEN]', got %s", config.RedactMessage)
	}
}

func TestGetPackages(t *testing.T) {
	// Test with packages specified
	config := &Config{
		Packages: []string{"./pkg1", "./pkg2"},
	}
	
	packages := config.GetPackages()
	if len(packages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(packages))
	}
	
	// Test with empty packages
	config = &Config{
		Packages: []string{},
	}
	
	packages = config.GetPackages()
	if len(packages) != 1 || packages[0] != "." {
		t.Errorf("Expected default package ['.'], got %v", packages)
	}
}
