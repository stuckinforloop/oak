package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/stuckinforloop/oak/internal/cli"
	"github.com/stuckinforloop/oak/internal/config"
	"github.com/stuckinforloop/oak/internal/generator"
	"github.com/stuckinforloop/oak/internal/parser"
	"github.com/stuckinforloop/oak/internal/writer"
)

var version string

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Parse command-line arguments
	opts, err := cli.ParseArgs(args)
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Handle help and version flags
	if opts.Help {
		printHelp()
		return nil
	}

	if opts.Version {
		fmt.Printf("oak %s\n", getBuildVersion())
		return nil
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine what to process
	target := opts.GetProcessingTarget()

	// Get the paths to process
	paths, err := getProcessingPaths(target, cfg)
	if err != nil {
		return fmt.Errorf("failed to determine processing paths: %w", err)
	}

	if len(paths) == 0 {
		return fmt.Errorf("no paths to process")
	}

	// Process each path
	var allStructs []parser.StructInfo
	oakParser := parser.New()

	for _, path := range paths {
		var result *parser.ParseResult
		var parseErr error

		if target.Mode == cli.ModeSourceFile {
			result, parseErr = oakParser.ParseFile(path)
		} else {
			result, parseErr = oakParser.ParsePackage(path)
		}

		if parseErr != nil {
			return fmt.Errorf("failed to parse %s: %w", path, parseErr)
		}

		allStructs = append(allStructs, result.Structs...)
	}

	if len(allStructs) == 0 {
		fmt.Println("No structs found with //go:generate oak directive")
		return nil
	}

	// Group structs by package
	packageStructs := groupStructsByPackage(allStructs)

	// Generate code for each package
	gen := generator.New(cfg)
	fileWriter := writer.New()

	var generatedFiles []string

	for packageName, structs := range packageStructs {
		result, err := gen.GenerateForStructs(structs)
		if err != nil {
			return fmt.Errorf("failed to generate code for package %s: %w", packageName, err)
		}

		if err := fileWriter.WriteResult(result); err != nil {
			return fmt.Errorf("failed to write generated file: %w", err)
		}

		generatedFiles = append(generatedFiles, result.FilePath)
	}

	fmt.Printf("Successfully processed %d struct(s) in %d package(s)\n",
		len(allStructs), len(packageStructs))

	return nil
}

func getBuildVersion() string {
	if version != "" {
		return version
	}

	if buildInfo, exists := debug.ReadBuildInfo(); exists {
		return buildInfo.Main.Version
	}

	return "unknown"
}

func getProcessingPaths(target *cli.ProcessingTarget, cfg *config.Config) ([]string, error) {
	switch target.Mode {
	case cli.ModeSourceFile, cli.ModePackage:
		// Use paths from flags
		return target.Paths, nil

	case cli.ModePositional:
		// Expand positional arguments
		return cli.ExpandPaths(target.Paths)

	case cli.ModeConfig:
		// Use paths from configuration
		return cli.ExpandPaths(cfg.GetPackages())

	default:
		return nil, fmt.Errorf("unknown processing mode")
	}
}

func groupStructsByPackage(structs []parser.StructInfo) map[string][]parser.StructInfo {
	groups := make(map[string][]parser.StructInfo)

	for _, s := range structs {
		groups[s.PackageName] = append(groups[s.PackageName], s)
	}

	return groups
}

func printHelp() {
	fmt.Printf(`oak %s - Go structured logging code generator

USAGE:
    oak [OPTIONS] [PATH]

DESCRIPTION:
    Oak generates LogValue() methods for Go structs to integrate with log/slog.
    It automatically handles type-specific logging, field redaction, and exclusion.

OPTIONS:
    --source <FILE>     Process a specific Go source file
    --package <DIR>     Process a specific package directory
    --help, -h          Show this help message
    --version, -v       Show version information

ARGUMENTS:
    PATH                Package path or directory to process
                        Use "./..." to process all packages recursively

EXAMPLES:
    oak                           Process current directory based on oak.yaml
    oak ./...                     Process all packages recursively
    oak ./internal/booking        Process specific package
    oak --package ./internal/booking
    oak --source ./booking.go     Process specific file

CONFIGURATION:
    Oak uses an oak.yaml file in the project root for configuration.
    See the example oak.yaml file for available options.

For more information, visit: https://github.com/stuckinforloop/oak
`, version)
}
