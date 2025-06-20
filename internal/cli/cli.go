package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options represents the parsed command-line options
type Options struct {
	// SourceFile is the path to a specific Go source file to process
	SourceFile string
	
	// PackagePath is the path to a package directory to process
	PackagePath string
	
	// PositionalArgs are the non-flag arguments (e.g., "./..." or "./pkg")
	PositionalArgs []string
	
	// Help indicates if help was requested
	Help bool
	
	// Version indicates if version was requested
	Version bool
}

// ProcessingMode represents how Oak should process files/packages
type ProcessingMode int

const (
	// ModeConfig processes packages based on oak.yaml configuration
	ModeConfig ProcessingMode = iota
	
	// ModeSourceFile processes a specific source file
	ModeSourceFile
	
	// ModePackage processes a specific package
	ModePackage
	
	// ModePositional processes based on positional arguments
	ModePositional
)

// ProcessingTarget represents what Oak should process
type ProcessingTarget struct {
	Mode     ProcessingMode
	Paths    []string
	UseFlags bool // true if flags were used, false if positional args
}

// ParseArgs parses command-line arguments and returns Options
func ParseArgs(args []string) (*Options, error) {
	opts := &Options{}
	
	// Create a custom flag set to avoid conflicts with testing
	fs := flag.NewFlagSet("oak", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: oak [options] [path]\n\n")
		fmt.Fprintf(fs.Output(), "Oak generates LogValue methods for Go structs to integrate with log/slog.\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  oak                           # Process current directory based on oak.yaml\n")
		fmt.Fprintf(fs.Output(), "  oak ./...                     # Process all packages recursively\n")
		fmt.Fprintf(fs.Output(), "  oak ./internal/booking        # Process specific package\n")
		fmt.Fprintf(fs.Output(), "  oak --package ./internal/booking\n")
		fmt.Fprintf(fs.Output(), "  oak --source ./booking.go     # Process specific file\n")
	}
	
	fs.StringVar(&opts.SourceFile, "source", "", "Path to a specific Go source file to process")
	fs.StringVar(&opts.PackagePath, "package", "", "Path to a package directory to process")
	fs.BoolVar(&opts.Help, "help", false, "Show help message")
	fs.BoolVar(&opts.Help, "h", false, "Show help message (shorthand)")
	fs.BoolVar(&opts.Version, "version", false, "Show version information")
	fs.BoolVar(&opts.Version, "v", false, "Show version information (shorthand)")
	
	// Parse the arguments
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	
	// Get remaining positional arguments
	opts.PositionalArgs = fs.Args()
	
	return opts, nil
}

// Validate checks the options for conflicts and invalid combinations
func (opts *Options) Validate() error {
	// Check for mutually exclusive flags
	if opts.SourceFile != "" && opts.PackagePath != "" {
		return fmt.Errorf("--source and --package flags cannot be used together")
	}
	
	// If flags are used, positional arguments should be ignored
	if (opts.SourceFile != "" || opts.PackagePath != "") && len(opts.PositionalArgs) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Positional arguments ignored when using flags\n")
	}
	
	// Validate source file exists if specified
	if opts.SourceFile != "" {
		if !strings.HasSuffix(opts.SourceFile, ".go") {
			return fmt.Errorf("source file must have .go extension: %s", opts.SourceFile)
		}
		if _, err := os.Stat(opts.SourceFile); os.IsNotExist(err) {
			return fmt.Errorf("source file does not exist: %s", opts.SourceFile)
		}
	}
	
	// Validate package path exists if specified
	if opts.PackagePath != "" {
		if _, err := os.Stat(opts.PackagePath); os.IsNotExist(err) {
			return fmt.Errorf("package path does not exist: %s", opts.PackagePath)
		}
	}
	
	// Validate positional arguments
	for _, arg := range opts.PositionalArgs {
		if arg != "./..." && arg != "." {
			// Check if it's a valid path
			if _, err := os.Stat(arg); os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", arg)
			}
		}
	}
	
	return nil
}

// GetProcessingTarget determines what Oak should process based on the options
func (opts *Options) GetProcessingTarget() *ProcessingTarget {
	target := &ProcessingTarget{}
	
	// Highest priority: flags
	if opts.SourceFile != "" {
		target.Mode = ModeSourceFile
		target.Paths = []string{opts.SourceFile}
		target.UseFlags = true
		return target
	}
	
	if opts.PackagePath != "" {
		target.Mode = ModePackage
		target.Paths = []string{opts.PackagePath}
		target.UseFlags = true
		return target
	}
	
	// Medium priority: positional arguments
	if len(opts.PositionalArgs) > 0 {
		target.Mode = ModePositional
		target.Paths = opts.PositionalArgs
		target.UseFlags = false
		return target
	}
	
	// Lowest priority: config-based processing
	target.Mode = ModeConfig
	target.Paths = []string{} // Will be determined by config
	target.UseFlags = false
	return target
}

// ExpandPaths expands path patterns like "./..." into actual package paths
func ExpandPaths(paths []string) ([]string, error) {
	var expanded []string
	
	for _, path := range paths {
		if path == "./..." {
			// Find all Go packages recursively
			packages, err := findGoPackages(".")
			if err != nil {
				return nil, fmt.Errorf("failed to expand %s: %w", path, err)
			}
			expanded = append(expanded, packages...)
		} else {
			expanded = append(expanded, path)
		}
	}
	
	return expanded, nil
}

// findGoPackages recursively finds all directories containing Go files
func findGoPackages(root string) ([]string, error) {
	var packages []string
	
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip hidden directories and vendor
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
		}
		
		// Check if this directory contains Go files
		if info.IsDir() {
			hasGoFiles, err := hasGoFilesInDir(path)
			if err != nil {
				return err
			}
			if hasGoFiles {
				packages = append(packages, path)
			}
		}
		
		return nil
	})
	
	return packages, err
}

// hasGoFilesInDir checks if a directory contains any .go files
func hasGoFilesInDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	
	return false, nil
}
