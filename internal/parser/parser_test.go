package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasOakDirective(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "has oak directive",
			content: `package main

//go:generate oak
type User struct {
	Name string
}`,
			expected: true,
		},
		{
			name: "has oak directive with extra text",
			content: `package main

//go:generate oak --verbose
type User struct {
	Name string
}`,
			expected: true,
		},
		{
			name: "no oak directive",
			content: `package main

type User struct {
	Name string
}`,
			expected: false,
		},
		{
			name: "has other go:generate directive",
			content: `package main

//go:generate mockgen
type User struct {
	Name string
}`,
			expected: false,
		},
		{
			name: "has oak directive in block comment",
			content: `package main

/*
go:generate oak
*/
type User struct {
	Name string
}`,
			expected: true,
		},
	}

	parser := New()
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "test.go")
			err := os.WriteFile(filePath, []byte(tc.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result, err := parser.ParseFile(filePath)
			if err != nil {
				t.Fatalf("Failed to parse file: %v", err)
			}

			hasStructs := len(result.Structs) > 0
			if hasStructs != tc.expected {
				t.Errorf("Expected hasOakDirective to be %v, got %v", tc.expected, hasStructs)
			}
		})
	}
}

func TestExtractStructs(t *testing.T) {
	content := `package booking

//go:generate oak
type Reservation struct {
	ID        int
	GuestName string
	Password  string
	Notes     string ` + "`log:\"-\"`" + `
	Secret    string ` + "`log:\"redact\"`" + `
	Pointer   *string
}

type AnotherStruct struct {
	Field1 string
	Field2 int
}`

	parser := New()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	if len(result.Structs) != 2 {
		t.Fatalf("Expected 2 structs, got %d", len(result.Structs))
	}

	// Check first struct (Reservation)
	reservation := result.Structs[0]
	if reservation.Name != "Reservation" {
		t.Errorf("Expected struct name 'Reservation', got %s", reservation.Name)
	}
	if reservation.PackageName != "booking" {
		t.Errorf("Expected package name 'booking', got %s", reservation.PackageName)
	}
	if len(reservation.Fields) != 6 {
		t.Errorf("Expected 6 fields, got %d", len(reservation.Fields))
	}

	// Check specific fields
	expectedFields := []struct {
		name      string
		fieldType string
		logTag    string
		isPointer bool
	}{
		{"ID", "int", "", false},
		{"GuestName", "string", "", false},
		{"Password", "string", "", false},
		{"Notes", "string", "-", false},
		{"Secret", "string", "redact", false},
		{"Pointer", "*string", "", true},
	}

	for i, expected := range expectedFields {
		if i >= len(reservation.Fields) {
			t.Errorf("Missing field %d", i)
			continue
		}
		field := reservation.Fields[i]
		if field.Name != expected.name {
			t.Errorf("Field %d: expected name %s, got %s", i, expected.name, field.Name)
		}
		if field.Type != expected.fieldType {
			t.Errorf("Field %d: expected type %s, got %s", i, expected.fieldType, field.Type)
		}
		if field.LogTag != expected.logTag {
			t.Errorf("Field %d: expected log tag %s, got %s", i, expected.logTag, field.LogTag)
		}
		if field.IsPointer != expected.isPointer {
			t.Errorf("Field %d: expected isPointer %v, got %v", i, expected.isPointer, field.IsPointer)
		}
	}
}

func TestExtractLogTag(t *testing.T) {
	parser := New()
	
	testCases := []struct {
		tagValue string
		expected string
	}{
		{"`log:\"-\"`", "-"},
		{"`log:\"redact\"`", "redact"},
		{"`json:\"name\" log:\"redact\"`", "redact"},
		{"`log:\"redact\" json:\"name\"`", "redact"},
		{"`json:\"name\"`", ""},
		{"", ""},
		{"`log:\"\"`", ""},
	}
	
	for _, tc := range testCases {
		result := parser.extractLogTag(tc.tagValue)
		if result != tc.expected {
			t.Errorf("extractLogTag(%s) = %s, expected %s", tc.tagValue, result, tc.expected)
		}
	}
}

func TestTypeToString(t *testing.T) {
	// This test would require creating AST nodes manually, which is complex
	// For now, we'll test it indirectly through the struct parsing tests
	// The TestExtractStructs test already validates type string conversion
}

func TestParsePackage(t *testing.T) {
	// Create a temporary package directory with multiple files
	tempDir := t.TempDir()
	
	// File with Oak directive
	file1Content := `package testpkg

//go:generate oak
type User struct {
	Name string
	Age  int
}`
	
	// File without Oak directive
	file2Content := `package testpkg

type Product struct {
	Name  string
	Price float64
}`
	
	// File with Oak directive
	file3Content := `package testpkg

//go:generate oak
type Order struct {
	ID     int
	UserID int
}`
	
	err := os.WriteFile(filepath.Join(tempDir, "user.go"), []byte(file1Content), 0644)
	if err != nil {
		t.Fatalf("Failed to create user.go: %v", err)
	}
	
	err = os.WriteFile(filepath.Join(tempDir, "product.go"), []byte(file2Content), 0644)
	if err != nil {
		t.Fatalf("Failed to create product.go: %v", err)
	}
	
	err = os.WriteFile(filepath.Join(tempDir, "order.go"), []byte(file3Content), 0644)
	if err != nil {
		t.Fatalf("Failed to create order.go: %v", err)
	}
	
	parser := New()
	result, err := parser.ParsePackage(tempDir)
	if err != nil {
		t.Fatalf("Failed to parse package: %v", err)
	}
	
	// Should find 2 structs (User and Order) since Product doesn't have Oak directive
	if len(result.Structs) != 2 {
		t.Errorf("Expected 2 structs, got %d", len(result.Structs))
	}
	
	// Check that we got the right structs
	structNames := make(map[string]bool)
	for _, s := range result.Structs {
		structNames[s.Name] = true
	}
	
	if !structNames["User"] {
		t.Errorf("Expected to find User struct")
	}
	if !structNames["Order"] {
		t.Errorf("Expected to find Order struct")
	}
	if structNames["Product"] {
		t.Errorf("Should not find Product struct (no Oak directive)")
	}
}
