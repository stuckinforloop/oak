package main

import (
	"log/slog"
	"os"
)

//go:generate oak
type Example struct {
	IntValue    int
	StringValue string
	FloatValue  float64
	NestedValue NestedExample
}

//go:generate oak
type NestedExample struct {
	BoolValue    bool
	SliceValue   []string
	PointerValue *string
}

func main() {
	example := Example{
		IntValue:    1,
		StringValue: "Hello, World!",
		FloatValue:  3.14,
		NestedValue: NestedExample{
			BoolValue:    true,
			SliceValue:   []string{"a", "b", "c"},
			PointerValue: toPtr("Hello, World!"),
		},
	}

	log := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{AddSource: true}),
	)
	log.Info("logging example", "example", example)

	/* Prints the following JSON to stdout
	{"time":"2025-06-20T20:43:05.59325+05:30","level":"INFO","source":{"function":"main.main","file":"/Users/stuckinforloop/dev/oak/example/main.go","line":40},"msg":"logging example","example":{"IntValue":1,"StringValue":"[REDACTED]","FloatValue":3.14,"NestedValue":{"BoolValue":"[REDACTED]","SliceValue":["a","b","c"],"PointerValue":"Hello, World!"}}}

	Formatted:
	{
		"time": "2025-06-20T20:43:25.289576+05:30",
		"level": "INFO",
		"source": {
			"function": "main.main",
			"file": "/Users/stuckinforloop/dev/oak/example/main.go",
			"line": 40
		},
		"msg": "logging example",
		"example": {
			"IntValue": 1,
			"StringValue": "[REDACTED]",
			"FloatValue": 3.14,
			"NestedValue": {
			"BoolValue": "[REDACTED]",
			"SliceValue": [
				"a",
				"b",
				"c"
			],
			"PointerValue": "Hello, World!"
			}
		}
	}
	*/
}

func toPtr[T any](t T) *T {
	return &t
}
