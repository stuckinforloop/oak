package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected *Options
		hasError bool
	}{
		{
			name: "no arguments",
			args: []string{},
			expected: &Options{
				PositionalArgs: []string{},
			},
		},
		{
			name: "source flag",
			args: []string{"--source", "main.go"},
			expected: &Options{
				SourceFile:     "main.go",
				PositionalArgs: []string{},
			},
		},
		{
			name: "package flag",
			args: []string{"--package", "./internal/booking"},
			expected: &Options{
				PackagePath:    "./internal/booking",
				PositionalArgs: []string{},
			},
		},
		{
			name: "positional argument",
			args: []string{"./..."},
			expected: &Options{
				PositionalArgs: []string{"./..."},
			},
		},
		{
			name: "multiple positional arguments",
			args: []string{"./pkg1", "./pkg2"},
			expected: &Options{
				PositionalArgs: []string{"./pkg1", "./pkg2"},
			},
		},
		{
			name: "help flag",
			args: []string{"--help"},
			expected: &Options{
				Help:           true,
				PositionalArgs: []string{},
			},
		},
		{
			name: "version flag",
			args: []string{"--version"},
			expected: &Options{
				Version:        true,
				PositionalArgs: []string{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := ParseArgs(tc.args)
			
			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if opts.SourceFile != tc.expected.SourceFile {
				t.Errorf("SourceFile: expected %s, got %s", tc.expected.SourceFile, opts.SourceFile)
			}
			
			if opts.PackagePath != tc.expected.PackagePath {
				t.Errorf("PackagePath: expected %s, got %s", tc.expected.PackagePath, opts.PackagePath)
			}
			
			if opts.Help != tc.expected.Help {
				t.Errorf("Help: expected %v, got %v", tc.expected.Help, opts.Help)
			}
			
			if opts.Version != tc.expected.Version {
				t.Errorf("Version: expected %v, got %v", tc.expected.Version, opts.Version)
			}
			
			if len(opts.PositionalArgs) != len(tc.expected.PositionalArgs) {
				t.Errorf("PositionalArgs length: expected %d, got %d", len(tc.expected.PositionalArgs), len(opts.PositionalArgs))
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// Create temporary files for testing
	tempDir := t.TempDir()
	testGoFile := filepath.Join(tempDir, "test.go")
	testDir := filepath.Join(tempDir, "testpkg")
	
	// Create test files
	os.WriteFile(testGoFile, []byte("package main"), 0644)
	os.MkdirAll(testDir, 0755)
	
	testCases := []struct {
		name      string
		opts      *Options
		hasError  bool
		errorMsg  string
	}{
		{
			name: "valid source file",
			opts: &Options{
				SourceFile: testGoFile,
			},
			hasError: false,
		},
		{
			name: "valid package path",
			opts: &Options{
				PackagePath: testDir,
			},
			hasError: false,
		},
		{
			name: "conflicting flags",
			opts: &Options{
				SourceFile:  testGoFile,
				PackagePath: testDir,
			},
			hasError: true,
			errorMsg: "--source and --package flags cannot be used together",
		},
		{
			name: "non-existent source file",
			opts: &Options{
				SourceFile: "/nonexistent/file.go",
			},
			hasError: true,
			errorMsg: "source file does not exist",
		},
		{
			name: "invalid source file extension",
			opts: &Options{
				SourceFile: "test.txt",
			},
			hasError: true,
			errorMsg: "source file must have .go extension",
		},
		{
			name: "non-existent package path",
			opts: &Options{
				PackagePath: "/nonexistent/package",
			},
			hasError: true,
			errorMsg: "package path does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			
			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tc.errorMsg != "" && !contains(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error to contain %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetProcessingTarget(t *testing.T) {
	testCases := []struct {
		name     string
		opts     *Options
		expected *ProcessingTarget
	}{
		{
			name: "source file mode",
			opts: &Options{
				SourceFile: "main.go",
			},
			expected: &ProcessingTarget{
				Mode:     ModeSourceFile,
				Paths:    []string{"main.go"},
				UseFlags: true,
			},
		},
		{
			name: "package mode",
			opts: &Options{
				PackagePath: "./internal/booking",
			},
			expected: &ProcessingTarget{
				Mode:     ModePackage,
				Paths:    []string{"./internal/booking"},
				UseFlags: true,
			},
		},
		{
			name: "positional mode",
			opts: &Options{
				PositionalArgs: []string{"./..."},
			},
			expected: &ProcessingTarget{
				Mode:     ModePositional,
				Paths:    []string{"./..."},
				UseFlags: false,
			},
		},
		{
			name: "config mode",
			opts: &Options{},
			expected: &ProcessingTarget{
				Mode:     ModeConfig,
				Paths:    []string{},
				UseFlags: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.opts.GetProcessingTarget()
			
			if target.Mode != tc.expected.Mode {
				t.Errorf("Mode: expected %v, got %v", tc.expected.Mode, target.Mode)
			}
			
			if target.UseFlags != tc.expected.UseFlags {
				t.Errorf("UseFlags: expected %v, got %v", tc.expected.UseFlags, target.UseFlags)
			}
			
			if len(target.Paths) != len(tc.expected.Paths) {
				t.Errorf("Paths length: expected %d, got %d", len(tc.expected.Paths), len(target.Paths))
			}
		})
	}
}

func TestHasGoFilesInDir(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()
	
	// Directory with Go files
	goDir := filepath.Join(tempDir, "withgo")
	os.MkdirAll(goDir, 0755)
	os.WriteFile(filepath.Join(goDir, "main.go"), []byte("package main"), 0644)
	
	// Directory without Go files
	noGoDir := filepath.Join(tempDir, "nogo")
	os.MkdirAll(noGoDir, 0755)
	os.WriteFile(filepath.Join(noGoDir, "readme.txt"), []byte("readme"), 0644)
	
	// Test directory with Go files
	hasGo, err := hasGoFilesInDir(goDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !hasGo {
		t.Errorf("Expected directory to have Go files")
	}
	
	// Test directory without Go files
	hasGo, err = hasGoFilesInDir(noGoDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if hasGo {
		t.Errorf("Expected directory to not have Go files")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}
