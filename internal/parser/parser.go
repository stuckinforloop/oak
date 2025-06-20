package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// StructInfo represents information about a struct that needs LogValue generation
type StructInfo struct {
	Name        string      // Name of the struct
	PackageName string      // Package name
	Fields      []FieldInfo // List of fields in the struct
	FilePath    string      // Path to the source file
}

// FieldInfo represents information about a struct field
type FieldInfo struct {
	Name     string // Field name
	Type     string // Field type as string
	Tag      string // Complete struct tag
	LogTag   string // Value of the log tag (e.g., "redact", "-")
	IsPointer bool  // Whether the field is a pointer type
}

// ParseResult represents the result of parsing Go source files
type ParseResult struct {
	Structs []StructInfo // Structs that need LogValue generation
	Errors  []error      // Any parsing errors encountered
}

// Parser handles parsing Go source files for Oak directives
type Parser struct {
	fileSet *token.FileSet
}

// New creates a new Parser instance
func New() *Parser {
	return &Parser{
		fileSet: token.NewFileSet(),
	}
}

// ParseFile parses a single Go source file for Oak directives
func (p *Parser) ParseFile(filePath string) (*ParseResult, error) {
	result := &ParseResult{}
	
	// Parse the Go source file
	file, err := parser.ParseFile(p.fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}
	
	// Check if the file has the //go:generate oak directive
	if !p.hasOakDirective(file) {
		return result, nil // No Oak directive found, return empty result
	}
	
	// Extract structs from the file
	structs := p.extractStructs(file, filePath)
	result.Structs = structs
	
	return result, nil
}

// ParsePackage parses all Go files in a package directory for Oak directives
func (p *Parser) ParsePackage(packagePath string) (*ParseResult, error) {
	result := &ParseResult{}
	
	// Parse all Go files in the package
	packages, err := parser.ParseDir(p.fileSet, packagePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package %s: %w", packagePath, err)
	}
	
	// Process each package (there should typically be only one)
	for _, pkg := range packages {
		for filePath, file := range pkg.Files {
			// Check if this file has the Oak directive
			if !p.hasOakDirective(file) {
				continue
			}
			
			// Extract structs from this file
			structs := p.extractStructs(file, filePath)
			result.Structs = append(result.Structs, structs...)
		}
	}
	
	return result, nil
}

// hasOakDirective checks if a file contains the //go:generate oak directive
func (p *Parser) hasOakDirective(file *ast.File) bool {
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			text := strings.TrimSpace(comment.Text)
			// Remove // or /* */ comment markers
			if strings.HasPrefix(text, "//") {
				text = strings.TrimSpace(text[2:])
			} else if strings.HasPrefix(text, "/*") && strings.HasSuffix(text, "*/") {
				text = strings.TrimSpace(text[2 : len(text)-2])
			}
			
			// Check for go:generate oak directive
			if strings.HasPrefix(text, "go:generate oak") {
				return true
			}
		}
	}
	return false
}

// extractStructs extracts all struct declarations from a file
func (p *Parser) extractStructs(file *ast.File, filePath string) []StructInfo {
	var structs []StructInfo
	
	// Walk the AST to find struct declarations
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.GenDecl:
			// Check if this is a type declaration
			if n.Tok == token.TYPE {
				for _, spec := range n.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							// Found a struct declaration
							structInfo := StructInfo{
								Name:        typeSpec.Name.Name,
								PackageName: file.Name.Name,
								FilePath:    filePath,
								Fields:      p.extractFields(structType),
							}
							structs = append(structs, structInfo)
						}
					}
				}
			}
		}
		return true
	})
	
	return structs
}

// extractFields extracts field information from a struct type
func (p *Parser) extractFields(structType *ast.StructType) []FieldInfo {
	var fields []FieldInfo
	
	for _, field := range structType.Fields.List {
		// Handle multiple names for the same type (e.g., x, y int)
		if len(field.Names) == 0 {
			// Anonymous field (embedded struct)
			fieldInfo := FieldInfo{
				Name:      p.typeToString(field.Type),
				Type:      p.typeToString(field.Type),
				IsPointer: p.isPointerType(field.Type),
			}
			if field.Tag != nil {
				fieldInfo.Tag = field.Tag.Value
				fieldInfo.LogTag = p.extractLogTag(field.Tag.Value)
			}
			fields = append(fields, fieldInfo)
		} else {
			for _, name := range field.Names {
				fieldInfo := FieldInfo{
					Name:      name.Name,
					Type:      p.typeToString(field.Type),
					IsPointer: p.isPointerType(field.Type),
				}
				if field.Tag != nil {
					fieldInfo.Tag = field.Tag.Value
					fieldInfo.LogTag = p.extractLogTag(field.Tag.Value)
				}
				fields = append(fields, fieldInfo)
			}
		}
	}
	
	return fields
}

// typeToString converts an AST type expression to a string representation
func (p *Parser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.typeToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.typeToString(t.Elt)
		}
		return "[" + p.typeToString(t.Len) + "]" + p.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + p.typeToString(t.Key) + "]" + p.typeToString(t.Value)
	case *ast.SelectorExpr:
		return p.typeToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}

// isPointerType checks if a type expression represents a pointer type
func (p *Parser) isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// extractLogTag extracts the value of the log struct tag
func (p *Parser) extractLogTag(tagValue string) string {
	// Remove backticks from tag value
	if len(tagValue) >= 2 && tagValue[0] == '`' && tagValue[len(tagValue)-1] == '`' {
		tagValue = tagValue[1 : len(tagValue)-1]
	}
	
	// Parse the tag to find the log tag
	// Simple parsing - look for log:"value"
	parts := strings.Split(tagValue, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "log:") {
			// Extract the value between quotes
			colonIndex := strings.Index(part, ":")
			if colonIndex >= 0 && colonIndex < len(part)-1 {
				value := part[colonIndex+1:]
				// Remove quotes
				if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
					return value[1 : len(value)-1]
				}
			}
		}
	}
	
	return ""
}

// GetAbsolutePath returns the absolute path for a given file path
func GetAbsolutePath(path string) (string, error) {
	return filepath.Abs(path)
}
