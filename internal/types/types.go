package types

import (
	"fmt"
	"strings"

	"github.com/stuckinforloop/oak/internal/config"
	"github.com/stuckinforloop/oak/internal/parser"
)

// SlogFunction represents the slog function to use for a field
type SlogFunction string

const (
	SlogInt64   SlogFunction = "slog.Int64"
	SlogString  SlogFunction = "slog.String"
	SlogBool    SlogFunction = "slog.Bool"
	SlogFloat64 SlogFunction = "slog.Float64"
	SlogAny     SlogFunction = "slog.Any"
)

// FieldAction represents what action to take for a field during logging
type FieldAction int

const (
	// ActionLog means the field should be logged normally
	ActionLog FieldAction = iota

	// ActionRedact means the field should be redacted
	ActionRedact

	// ActionSkip means the field should be skipped entirely
	ActionSkip
)

// FieldAnalysis contains the analysis result for a struct field
type FieldAnalysis struct {
	Field    parser.FieldInfo // Original field information
	Action   FieldAction      // What action to take
	SlogFunc SlogFunction     // Which slog function to use
	LogValue string           // The value to log (for redacted fields)
}

// TypeAnalyzer analyzes struct fields and determines appropriate slog functions
type TypeAnalyzer struct {
	config *config.Config
}

// NewTypeAnalyzer creates a new TypeAnalyzer with the given configuration
func NewTypeAnalyzer(cfg *config.Config) *TypeAnalyzer {
	return &TypeAnalyzer{
		config: cfg,
	}
}

// AnalyzeField analyzes a single field and returns the appropriate analysis
func (ta *TypeAnalyzer) AnalyzeField(field parser.FieldInfo) FieldAnalysis {
	analysis := FieldAnalysis{
		Field: field,
	}

	// First, check if the field should be skipped
	if field.LogTag == "-" {
		analysis.Action = ActionSkip
		return analysis
	}

	// Check if the field should be redacted
	if ta.shouldRedactField(field) {
		analysis.Action = ActionRedact
		analysis.SlogFunc = SlogString
		analysis.LogValue = ta.config.RedactMessage
		return analysis
	}

	// Field should be logged normally
	analysis.Action = ActionLog
	analysis.SlogFunc = ta.getSlogFunction(field)

	return analysis
}

// AnalyzeStruct analyzes all fields in a struct and returns field analyses
func (ta *TypeAnalyzer) AnalyzeStruct(structInfo parser.StructInfo) []FieldAnalysis {
	var analyses []FieldAnalysis

	for _, field := range structInfo.Fields {
		analysis := ta.AnalyzeField(field)
		analyses = append(analyses, analysis)
	}

	return analyses
}

// shouldRedactField determines if a field should be redacted
func (ta *TypeAnalyzer) shouldRedactField(field parser.FieldInfo) bool {
	// Skip fields should not be redacted (they're handled separately)
	if field.LogTag == "-" {
		return false
	}

	// Check explicit log:"redact" tag
	if field.LogTag == "redact" {
		return true
	}

	// Check if field name matches redaction keys (case-insensitive)
	return ta.config.ShouldRedactField(field.Name)
}

// getSlogFunction determines the appropriate slog function for a field type
func (ta *TypeAnalyzer) getSlogFunction(field parser.FieldInfo) SlogFunction {
	// Handle pointer types - we'll dereference them in the template
	fieldType := field.Type
	if field.IsPointer {
		// Remove the * prefix for type analysis
		fieldType = strings.TrimPrefix(fieldType, "*")
	}

	// Map Go types to slog functions
	switch fieldType {
	// Integer types
	case "int", "int8", "int16", "int32", "int64":
		return SlogInt64
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return SlogInt64

	// String types
	case "string":
		return SlogString

	// Boolean types
	case "bool":
		return SlogBool

	// Floating-point types
	case "float32", "float64":
		return SlogFloat64

	// Complex types (structs, slices, maps, interfaces, etc.)
	default:
		return SlogAny
	}
}

// GenerateLogStatement generates the slog statement for a field
func (ta *TypeAnalyzer) GenerateLogStatement(analysis FieldAnalysis, receiverName string) string {
	fieldName := analysis.Field.Name

	switch analysis.Action {
	case ActionSkip:
		return "" // Field should not appear in log output

	case ActionRedact:
		return fmt.Sprintf(`%s("%s", "%s")`, analysis.SlogFunc, fieldName, analysis.LogValue)

	case ActionLog:
		return ta.generateNormalLogStatement(analysis, receiverName)

	default:
		return fmt.Sprintf(`%s("%s", %s)`, SlogAny, fieldName, ta.getFieldAccessor(analysis.Field, receiverName))
	}
}

// generateNormalLogStatement generates a normal (non-redacted) log statement
func (ta *TypeAnalyzer) generateNormalLogStatement(analysis FieldAnalysis, receiverName string) string {
	fieldName := analysis.Field.Name
	fieldAccessor := ta.getFieldAccessor(analysis.Field, receiverName)

	switch analysis.SlogFunc {
	case SlogInt64:
		if analysis.Field.IsPointer {
			// For pointer types, we need to handle nil case and convert to int64
			return fmt.Sprintf(`func() slog.Attr {
				if %s == nil {
					return slog.String("%s", "null")
				}
				return slog.Int64("%s", int64(*%s))
			}()`, fieldAccessor, fieldName, fieldName, fieldAccessor)
		}
		// For non-pointer integer types, convert to int64
		if analysis.Field.Type != "int64" {
			return fmt.Sprintf(`%s("%s", int64(%s))`, analysis.SlogFunc, fieldName, fieldAccessor)
		}
		return fmt.Sprintf(`%s("%s", %s)`, analysis.SlogFunc, fieldName, fieldAccessor)

	case SlogFloat64:
		if analysis.Field.IsPointer {
			return fmt.Sprintf(`func() slog.Attr {
				if %s == nil {
					return slog.String("%s", "null")
				}
				return slog.Float64("%s", float64(*%s))
			}()`, fieldAccessor, fieldName, fieldName, fieldAccessor)
		}
		// For non-pointer float types, convert to float64
		if analysis.Field.Type != "float64" {
			return fmt.Sprintf(`%s("%s", float64(%s))`, analysis.SlogFunc, fieldName, fieldAccessor)
		}
		return fmt.Sprintf(`%s("%s", %s)`, analysis.SlogFunc, fieldName, fieldAccessor)

	case SlogString, SlogBool:
		if analysis.Field.IsPointer {
			return fmt.Sprintf(`func() slog.Attr {
				if %s == nil {
					return slog.String("%s", "null")
				}
				return %s("%s", *%s)
			}()`, fieldAccessor, fieldName, analysis.SlogFunc, fieldName, fieldAccessor)
		}
		return fmt.Sprintf(`%s("%s", %s)`, analysis.SlogFunc, fieldName, fieldAccessor)

	case SlogAny:
		if analysis.Field.IsPointer {
			return fmt.Sprintf(`func() slog.Attr {
				if %s == nil {
					return slog.String("%s", "null")
				}
				return %s("%s", *%s)
			}()`, fieldAccessor, fieldName, analysis.SlogFunc, fieldName, fieldAccessor)
		}
		return fmt.Sprintf(`%s("%s", %s)`, analysis.SlogFunc, fieldName, fieldAccessor)

	default:
		return fmt.Sprintf(`%s("%s", %s)`, SlogAny, fieldName, fieldAccessor)
	}
}

// getFieldAccessor returns the Go code to access a field (e.g., "s.FieldName")
func (ta *TypeAnalyzer) getFieldAccessor(field parser.FieldInfo, receiverName string) string {
	return fmt.Sprintf("%s.%s", receiverName, field.Name)
}

// HasLoggableFields checks if a struct has any fields that should be logged
func (ta *TypeAnalyzer) HasLoggableFields(structInfo parser.StructInfo) bool {
	analyses := ta.AnalyzeStruct(structInfo)

	for _, analysis := range analyses {
		if analysis.Action != ActionSkip {
			return true
		}
	}

	return false
}
