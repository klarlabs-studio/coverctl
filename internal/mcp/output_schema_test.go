package mcp

import (
	"testing"

	"go.klarlabs.de/mcp/schema"
)

// TestOutputSchemasGenerate guards every type advertised via
// ToolBuilder.OutputSchema. OutputSchema runs schema.Generate at registration
// time and, if generation errors, silently drops the tool from the server.
// This test fails loudly instead, so a future change to an output struct (or a
// nested domain/application type) that breaks schema generation is caught in CI
// rather than by a tool mysteriously disappearing from tools/list.
func TestOutputSchemasGenerate(t *testing.T) {
	tests := []struct {
		name string
		typ  any
	}{
		{"check/report (ToolOutput)", ToolOutput{}},
		{"suggest (SuggestOutput)", SuggestOutput{}},
		{"debt (DebtOutput)", DebtOutput{}},
		{"compare (CompareOutput)", CompareOutput{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.Generate(tt.typ)
			if err != nil {
				t.Fatalf("schema.Generate(%T) returned error: %v", tt.typ, err)
			}
			if s == nil {
				t.Fatalf("schema.Generate(%T) returned nil schema", tt.typ)
			}
			if s.Type != "object" {
				t.Fatalf("schema.Generate(%T) type = %q, want object", tt.typ, s.Type)
			}
		})
	}
}
