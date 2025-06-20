package types

import (
	"testing"

	"github.com/stuckinforloop/oak/internal/config"
	"github.com/stuckinforloop/oak/internal/parser"
)

func TestGetSlogFunction(t *testing.T) {
	cfg := config.DefaultConfig()
	analyzer := NewTypeAnalyzer(cfg)

	testCases := []struct {
		fieldType string
		isPointer bool
		expected  SlogFunction
	}{
		// Integer types
		{"int", false, SlogInt64},
		{"int8", false, SlogInt64},
		{"int16", false, SlogInt64},
		{"int32", false, SlogInt64},
		{"int64", false, SlogInt64},
		{"uint", false, SlogInt64},
		{"uint8", false, SlogInt64},
		{"uint16", false, SlogInt64},
		{"uint32", false, SlogInt64},
		{"uint64", false, SlogInt64},

		// Pointer integer types
		{"*int", true, SlogInt64},
		{"*int64", true, SlogInt64},

		// String types
		{"string", false, SlogString},
		{"*string", true, SlogString},

		// Boolean types
		{"bool", false, SlogBool},
		{"*bool", true, SlogBool},

		// Float types
		{"float32", false, SlogFloat64},
		{"float64", false, SlogFloat64},
		{"*float64", true, SlogFloat64},

		// Complex types
		{"[]string", false, SlogAny},
		{"map[string]int", false, SlogAny},
		{"interface{}", false, SlogAny},
		{"CustomStruct", false, SlogAny},
	}

	for _, tc := range testCases {
		field := parser.FieldInfo{
			Name:      "TestField",
			Type:      tc.fieldType,
			IsPointer: tc.isPointer,
		}

		result := analyzer.getSlogFunction(field)
		if result != tc.expected {
			t.Errorf("getSlogFunction(%s, pointer=%v) = %s, expected %s",
				tc.fieldType, tc.isPointer, result, tc.expected)
		}
	}
}

func TestShouldRedactField(t *testing.T) {
	cfg := &config.Config{
		RedactKeys:    []string{"password", "secret", "api_key"},
		RedactMessage: "[REDACTED]",
	}
	analyzer := NewTypeAnalyzer(cfg)

	testCases := []struct {
		field    parser.FieldInfo
		expected bool
	}{
		// Explicit redact tag
		{parser.FieldInfo{Name: "Username", LogTag: "redact"}, true},

		// Redact keys (case-insensitive)
		{parser.FieldInfo{Name: "password"}, true},
		{parser.FieldInfo{Name: "Password"}, true},
		{parser.FieldInfo{Name: "PASSWORD"}, true},
		{parser.FieldInfo{Name: "secret"}, true},
		{parser.FieldInfo{Name: "Secret"}, true},
		{parser.FieldInfo{Name: "api_key"}, true},
		{parser.FieldInfo{Name: "API_KEY"}, true},

		// Non-redacted fields
		{parser.FieldInfo{Name: "username"}, false},
		{parser.FieldInfo{Name: "email"}, false},
		{parser.FieldInfo{Name: "id"}, false},

		// Skip tag takes precedence (should not be redacted)
		{parser.FieldInfo{Name: "password", LogTag: "-"}, false},
	}

	for _, tc := range testCases {
		result := analyzer.shouldRedactField(tc.field)
		if result != tc.expected {
			t.Errorf("shouldRedactField(%s, tag=%s) = %v, expected %v",
				tc.field.Name, tc.field.LogTag, result, tc.expected)
		}
	}
}

func TestAnalyzeField(t *testing.T) {
	cfg := &config.Config{
		RedactKeys:    []string{"password"},
		RedactMessage: "[HIDDEN]",
	}
	analyzer := NewTypeAnalyzer(cfg)

	testCases := []struct {
		name     string
		field    parser.FieldInfo
		expected FieldAnalysis
	}{
		{
			name: "normal string field",
			field: parser.FieldInfo{
				Name: "Username",
				Type: "string",
			},
			expected: FieldAnalysis{
				Action:   ActionLog,
				SlogFunc: SlogString,
			},
		},
		{
			name: "redacted field by name",
			field: parser.FieldInfo{
				Name: "Password",
				Type: "string",
			},
			expected: FieldAnalysis{
				Action:   ActionRedact,
				SlogFunc: SlogString,
				LogValue: "[HIDDEN]",
			},
		},
		{
			name: "redacted field by tag",
			field: parser.FieldInfo{
				Name:   "Token",
				Type:   "string",
				LogTag: "redact",
			},
			expected: FieldAnalysis{
				Action:   ActionRedact,
				SlogFunc: SlogString,
				LogValue: "[HIDDEN]",
			},
		},
		{
			name: "skipped field",
			field: parser.FieldInfo{
				Name:   "Notes",
				Type:   "string",
				LogTag: "-",
			},
			expected: FieldAnalysis{
				Action: ActionSkip,
			},
		},
		{
			name: "integer field",
			field: parser.FieldInfo{
				Name: "Age",
				Type: "int",
			},
			expected: FieldAnalysis{
				Action:   ActionLog,
				SlogFunc: SlogInt64,
			},
		},
		{
			name: "pointer field",
			field: parser.FieldInfo{
				Name:      "Email",
				Type:      "*string",
				IsPointer: true,
			},
			expected: FieldAnalysis{
				Action:   ActionLog,
				SlogFunc: SlogString,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.AnalyzeField(tc.field)

			if result.Action != tc.expected.Action {
				t.Errorf("Action: expected %v, got %v", tc.expected.Action, result.Action)
			}

			if result.SlogFunc != tc.expected.SlogFunc {
				t.Errorf("SlogFunc: expected %v, got %v", tc.expected.SlogFunc, result.SlogFunc)
			}

			if result.LogValue != tc.expected.LogValue {
				t.Errorf("LogValue: expected %v, got %v", tc.expected.LogValue, result.LogValue)
			}
		})
	}
}

func TestAnalyzeStruct(t *testing.T) {
	cfg := &config.Config{
		RedactKeys:    []string{"password"},
		RedactMessage: "[REDACTED]",
	}
	analyzer := NewTypeAnalyzer(cfg)

	structInfo := parser.StructInfo{
		Name:        "User",
		PackageName: "main",
		Fields: []parser.FieldInfo{
			{Name: "ID", Type: "int"},
			{Name: "Username", Type: "string"},
			{Name: "Password", Type: "string"},
			{Name: "Notes", Type: "string", LogTag: "-"},
			{Name: "Token", Type: "string", LogTag: "redact"},
		},
	}

	analyses := analyzer.AnalyzeStruct(structInfo)

	if len(analyses) != 5 {
		t.Fatalf("Expected 5 field analyses, got %d", len(analyses))
	}

	// Check specific field analyses
	expectedActions := []FieldAction{
		ActionLog,    // ID
		ActionLog,    // Username
		ActionRedact, // Password (matches redact key)
		ActionSkip,   // Notes (log:"-")
		ActionRedact, // Token (log:"redact")
	}

	for i, expected := range expectedActions {
		if analyses[i].Action != expected {
			t.Errorf("Field %d (%s): expected action %v, got %v",
				i, analyses[i].Field.Name, expected, analyses[i].Action)
		}
	}
}

func TestHasLoggableFields(t *testing.T) {
	cfg := config.DefaultConfig()
	analyzer := NewTypeAnalyzer(cfg)

	testCases := []struct {
		name     string
		fields   []parser.FieldInfo
		expected bool
	}{
		{
			name: "has loggable fields",
			fields: []parser.FieldInfo{
				{Name: "ID", Type: "int"},
				{Name: "Name", Type: "string"},
			},
			expected: true,
		},
		{
			name: "all fields skipped",
			fields: []parser.FieldInfo{
				{Name: "Field1", Type: "string", LogTag: "-"},
				{Name: "Field2", Type: "int", LogTag: "-"},
			},
			expected: false,
		},
		{
			name: "mixed fields",
			fields: []parser.FieldInfo{
				{Name: "ID", Type: "int"},
				{Name: "Notes", Type: "string", LogTag: "-"},
			},
			expected: true,
		},
		{
			name:     "no fields",
			fields:   []parser.FieldInfo{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			structInfo := parser.StructInfo{
				Name:   "TestStruct",
				Fields: tc.fields,
			}

			result := analyzer.HasLoggableFields(structInfo)
			if result != tc.expected {
				t.Errorf("HasLoggableFields() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestGenerateLogStatement(t *testing.T) {
	cfg := &config.Config{
		RedactMessage: "[HIDDEN]",
	}
	analyzer := NewTypeAnalyzer(cfg)

	testCases := []struct {
		name     string
		analysis FieldAnalysis
		expected string
	}{
		{
			name: "skipped field",
			analysis: FieldAnalysis{
				Field:  parser.FieldInfo{Name: "Notes"},
				Action: ActionSkip,
			},
			expected: "",
		},
		{
			name: "redacted field",
			analysis: FieldAnalysis{
				Field:    parser.FieldInfo{Name: "Password"},
				Action:   ActionRedact,
				SlogFunc: SlogString,
				LogValue: "[HIDDEN]",
			},
			expected: `slog.String("Password", "[HIDDEN]")`,
		},
		{
			name: "normal string field",
			analysis: FieldAnalysis{
				Field:    parser.FieldInfo{Name: "Username", Type: "string"},
				Action:   ActionLog,
				SlogFunc: SlogString,
			},
			expected: `slog.String("Username", u.Username)`,
		},
		{
			name: "normal int field",
			analysis: FieldAnalysis{
				Field:    parser.FieldInfo{Name: "Age", Type: "int"},
				Action:   ActionLog,
				SlogFunc: SlogInt64,
			},
			expected: `slog.Int64("Age", int64(u.Age))`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.GenerateLogStatement(tc.analysis, "u")
			if result != tc.expected {
				t.Errorf("GenerateLogStatement() = %q, expected %q", result, tc.expected)
			}
		})
	}
}
