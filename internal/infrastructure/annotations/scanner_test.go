package annotations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScannerDetectsAnnotations(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "main.go")
	content := `package main

// coverctl:domain=core
// coverctl:ignore
func main() {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := (Scanner{}).Scan(context.Background(), tmp, []string{"main.go"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ann, ok := out["main.go"]
	if !ok {
		t.Fatalf("expected annotation entry")
	}
	if !ann.Ignore {
		t.Fatalf("expected ignore true")
	}
	if ann.Domain != "core" {
		t.Fatalf("expected domain core, got %s", ann.Domain)
	}
}

func TestScannerDetectsPythonAnnotations(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "main.py")
	content := `# coverctl:domain=api
# coverctl:ignore
def main():
    pass
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := (Scanner{}).Scan(context.Background(), tmp, []string{"main.py"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ann, ok := out["main.py"]
	if !ok {
		t.Fatalf("expected annotation entry for .py file")
	}
	if !ann.Ignore {
		t.Fatalf("expected ignore true")
	}
	if ann.Domain != "api" {
		t.Fatalf("expected domain api, got %s", ann.Domain)
	}
}

func TestScannerDetectsTypeScriptAnnotations(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "index.ts")
	content := `// coverctl:ignore
export function main() {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := (Scanner{}).Scan(context.Background(), tmp, []string{"index.ts"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ann, ok := out["index.ts"]
	if !ok {
		t.Fatalf("expected annotation entry for .ts file")
	}
	if !ann.Ignore {
		t.Fatalf("expected ignore true")
	}
}

func TestScannerSkipsUnsupportedExtensions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "data.csv")
	if err := os.WriteFile(path, []byte("coverctl:ignore\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := (Scanner{}).Scan(context.Background(), tmp, []string{"data.csv"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no annotations for unsupported extension, got %d", len(out))
	}
}

func TestScannerIgnoresMissingFile(t *testing.T) {
	tmp := t.TempDir()
	if _, err := (Scanner{}).Scan(context.Background(), tmp, []string{"missing.go"}); err != nil {
		t.Fatalf("expected missing file to be ignored: %v", err)
	}
}

func TestScannerSkipsPathEscapingModuleRoot(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "module")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A secret file OUTSIDE the module root that carries an annotation. A
	// coverage-map-derived `../secret.go` entry must not read it.
	outside := filepath.Join(root, "secret.go")
	if err := os.WriteFile(outside, []byte("// coverctl:domain=secret\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := (Scanner{}).Scan(context.Background(), moduleRoot, []string{"../secret.go"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected escaping path to be skipped, got %v", out)
	}
	if _, ok := out["../secret.go"]; ok {
		t.Fatal("escaping path must not be read")
	}
}

func TestScannerSkipsWhenModuleRootEmpty(t *testing.T) {
	// Without a module root, containment cannot be enforced, so reads are skipped
	// rather than joining an attacker-influenced path freely.
	out, err := (Scanner{}).Scan(context.Background(), "", []string{"main.go", "../../etc/passwd.go"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no annotations without a module root, got %v", out)
	}
}
