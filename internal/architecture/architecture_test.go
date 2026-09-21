// Package architecture houses fitness functions: automated tests that
// enforce architectural decisions.
//
// The premise (Parsons/Ford/Kua, Building Evolutionary Architectures): if a
// rule lives only in a human's head or a code-review checklist, it will be
// violated. Encode it as a test, run it in CI, and the architecture stays
// honest under change. Failing here is not a bug in the test — it's a bug
// in the change that triggered it. The test message says what to do.
package architecture_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root, located by
// walking up from the test binary's working directory until we find go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from %s", dir)
		}
		dir = parent
	}
}

// importsOf returns the import paths of every Go file under root, mapped by
// the file's path relative to root. Skips _test.go files (tests can import
// anything; the rule is about production code shape).
func importsOf(t *testing.T, root string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		paths := make([]string, 0, len(f.Imports))
		for _, imp := range f.Imports {
			// imp.Path.Value is quoted: `"github.com/x/y"`
			paths = append(paths, strings.Trim(imp.Path.Value, `"`))
		}
		out[rel] = paths
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// TestLayerBoundary_DomainStaysClean asserts no file under internal/domain
// imports an outer layer. Domain is the innermost ring of the DDD onion;
// dependencies must point inward, not outward (Martin, Clean Architecture).
func TestLayerBoundary_DomainStaysClean(t *testing.T) {
	root := repoRoot(t)
	imports := importsOf(t, filepath.Join(root, "internal", "domain"))

	forbidden := []string{
		"go.klarlabs.de/coverctl/internal/application",
		"go.klarlabs.de/coverctl/internal/infrastructure",
		"go.klarlabs.de/coverctl/internal/cli",
		"go.klarlabs.de/coverctl/internal/mcp",
	}
	for file, paths := range imports {
		for _, p := range paths {
			for _, f := range forbidden {
				if strings.HasPrefix(p, f) {
					t.Errorf("internal/domain/%s imports %q; domain must not depend on outer layers", file, p)
				}
			}
		}
	}
}

// TestLayerBoundary_ApplicationStaysClean asserts no file under
// internal/application imports infrastructure, cli, or mcp. Application
// orchestrates the domain via interfaces it owns; infrastructure adapts those
// interfaces to specific tech. Reversing this dependency means the
// orchestration layer knows about a specific runner / parser / transport,
// which defeats the point of the abstraction.
func TestLayerBoundary_ApplicationStaysClean(t *testing.T) {
	root := repoRoot(t)
	imports := importsOf(t, filepath.Join(root, "internal", "application"))

	forbidden := []string{
		"go.klarlabs.de/coverctl/internal/infrastructure",
		"go.klarlabs.de/coverctl/internal/cli",
		"go.klarlabs.de/coverctl/internal/mcp",
	}
	for file, paths := range imports {
		for _, p := range paths {
			for _, f := range forbidden {
				if strings.HasPrefix(p, f) {
					t.Errorf("internal/application/%s imports %q; application must not depend on infrastructure / cli / mcp", file, p)
				}
			}
		}
	}
}

// fileSizeCeiling is a maintenance signal, not an architectural boundary.
// Hitting the ceiling means the next change to that file should extract
// before piling on; it does not encode a domain invariant. Dependency
// direction and forbidden coupling are the architectural tests above.
//
// Ceilings are set 5-10% above current line counts so this commit doesn't
// trip the test, but any growth fails immediately. Lower the ceiling
// whenever the file shrinks from refactoring.
type fileSizeCeiling struct {
	relpath string
	maxLOC  int
	reason  string
}

func TestFileSizeCeilings(t *testing.T) {
	root := repoRoot(t)

	ceilings := []fileSizeCeiling{
		{
			relpath: "internal/cli/cli.go",
			maxLOC:  490,
			reason:  "Dispatch is now a thin switch; helpers (help text, completion, printers, watch loop) live in cmd_*.go / version.go. Adding back inline bodies is the regression to prevent.",
		},
		{
			relpath: "internal/application/service.go",
			maxLOC:  1400,
			reason:  "Service god-struct with extracted handlers living alongside. Aggregate helpers moved to aggregate.go; migrate remaining methods to per-concern handlers (engineering review R1).",
		},
		{
			relpath: "internal/mcp/server.go",
			maxLOC:  1000,
			reason:  "Combined server bootstrap + 9 tool handlers + 4 resource handlers. Extract handlers to per-tool files when adding new MCP capabilities.",
		},
	}

	for _, c := range ceilings {
		t.Run(c.relpath, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, c.relpath))
			if err != nil {
				t.Fatalf("read %s: %v", c.relpath, err)
			}
			loc := strings.Count(string(data), "\n")
			if loc > c.maxLOC {
				t.Errorf("%s is %d lines, ceiling is %d.\n"+
					"REASON: %s\n"+
					"FIX: extract before adding more. Do not raise the ceiling without doing the extraction first.",
					c.relpath, loc, c.maxLOC, c.reason)
			}
		})
	}
}

// TestLayerBoundary_DomainImportsStayStdlib asserts production domain code
// only imports the Go standard library. Language runners, MCP, CLI
// frameworks, and infrastructure adapters must not leak into the policy
// engine — adding a language should not require domain changes.
func TestLayerBoundary_DomainImportsStayStdlib(t *testing.T) {
	root := repoRoot(t)
	imports := importsOf(t, filepath.Join(root, "internal", "domain"))
	for file, paths := range imports {
		for _, p := range paths {
			if strings.Contains(p, ".") {
				t.Errorf("internal/domain/%s imports %q; domain must stay language-agnostic (stdlib only)", file, p)
			}
			if p == "os/exec" || p == "net/http" || p == "plugin" {
				t.Errorf("internal/domain/%s imports %q; domain must not reach process/network/plugin surfaces", file, p)
			}
		}
	}
}

// TestLayerBoundary_ApplicationHasNoRunnerOrParserImports is belt-and-
// suspenders on the existing application-stays-clean rule: even a relative
// or renamed import of a language runner/parser into application would
// let toolchain details leak into orchestration.
func TestLayerBoundary_ApplicationHasNoRunnerOrParserImports(t *testing.T) {
	root := repoRoot(t)
	imports := importsOf(t, filepath.Join(root, "internal", "application"))
	forbidden := []string{
		"/runners",
		"/parsers",
		"/gotool",
		"os/exec",
	}
	for file, paths := range imports {
		for _, p := range paths {
			for _, f := range forbidden {
				if strings.Contains(p, f) {
					t.Errorf("internal/application/%s imports %q; runners/parsers/exec belong in infrastructure", file, p)
				}
			}
		}
	}
}

// TestSchemaLanguageEnumMatchesConstants asserts the JSON Schema's
// `language` enum is in lockstep with application.Language constants.
// Drift here was the user-visible bug surfaced in the UX review: --language
// flag advertised "nodejs" but schema rejected anything but "javascript".
//
// Keeps the schema authoritative for one canonical set of language names.
func TestSchemaLanguageEnumMatchesConstants(t *testing.T) {
	root := repoRoot(t)

	schemaBytes, err := os.ReadFile(filepath.Join(root, "schemas", "coverctl.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Properties struct {
			Language struct {
				Enum []string `json:"enum"`
			} `json:"language"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	typesPath := filepath.Join(root, "internal", "application", "types.go")
	typesBytes, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("read types.go: %v", err)
	}
	source := string(typesBytes)

	// Each schema enum value must be the literal of some Language constant.
	for _, lang := range schema.Properties.Language.Enum {
		needle := `Language = "` + lang + `"`
		if !strings.Contains(source, needle) {
			t.Errorf("schema enum value %q has no matching `Language = %q` constant in internal/application/types.go", lang, lang)
		}
	}

	// Every Language constant whose value is a real language (not the empty
	// string) must appear in the schema enum. Walk the source for assignments
	// of the form `LanguageXxx Language = "yyy"`.
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, `Language = "`)
		if idx == -1 {
			continue
		}
		rest := line[idx+len(`Language = "`):]
		end := strings.Index(rest, `"`)
		if end == -1 {
			continue
		}
		value := rest[:end]
		if value == "" {
			continue
		}
		found := false
		for _, e := range schema.Properties.Language.Enum {
			if e == value {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Language constant value %q is missing from schemas/coverctl.schema.json properties.language.enum", value)
		}
	}
}

// TestLanguageRegistryIsCompleteAndConsistent asserts the canonical
// application.Languages registry stays the single source of truth for
// language metadata. Drift between registry, runners, schema, and detector
// was the engineering-review R4 finding (8-place shotgun surgery to add a
// language). Now the rules are: every Language constant must appear once,
// every entry must declare extensions + format, and every entry must have
// markers OR be explicitly noted as marker-less (currently only Shell).
func TestLanguageRegistryIsCompleteAndConsistent(t *testing.T) {
	root := repoRoot(t)
	typesPath := filepath.Join(root, "internal", "application", "types.go")
	data, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("read types.go: %v", err)
	}
	source := string(data)

	// Every Language constant whose value is non-empty (not LanguageAuto)
	// must appear in the Languages registry.
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		// Match: LanguageX Language = "value"
		constIdx := strings.Index(line, " Language = \"")
		if constIdx == -1 {
			continue
		}
		name := strings.TrimSpace(line[:constIdx])
		if !strings.HasPrefix(name, "Language") {
			continue
		}
		rest := line[constIdx+len(` Language = "`):]
		end := strings.Index(rest, `"`)
		if end == -1 {
			continue
		}
		value := rest[:end]
		if value == "" || value == "auto" {
			continue
		}
		needle := "Code:             " + name
		if !strings.Contains(source, needle) {
			t.Errorf("Language %q (%s) missing from Languages registry (looked for %q)", value, name, needle)
		}
	}
}

// TestNoProductionCodeUsesTestOnlyHTTPConstructors prevents reintroduction
// of the SSRF / token-exfiltration sink that NewClientWithHTTP would open
// if reached from production code.
//
// NewClientWithHTTP accepts an arbitrary apiURL alongside a Bearer token.
// In tests this is fine — the URL points at httptest.Server. In production,
// any path that lets user input (CLI flag, MCP input, config field) reach
// that URL would let an attacker redirect the request to a host they
// control and harvest the token. Pin the rule: only _test.go files may
// reference NewClientWithHTTP.
func TestNoProductionCodeUsesTestOnlyHTTPConstructors(t *testing.T) {
	root := repoRoot(t)
	productionDirs := []string{
		filepath.Join(root, "internal", "cli"),
		filepath.Join(root, "internal", "mcp"),
		filepath.Join(root, "internal", "application"),
	}

	for _, dir := range productionDirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "NewClientWithHTTP") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s references NewClientWithHTTP, which is test-only. "+
					"Use NewClient (pinned API URL) in production code.", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

// TestNoDuplicatePRCommentDispatch guards against re-introducing the
// duplicate PR-comment workflow that lived in both PRCommentHandler and
// Service.PRComment. The fix was to make Service.PRComment delegate to the
// handler. If a future change resurrects the inline workflow in service.go,
// fail loudly.
func TestNoDuplicatePRCommentDispatch(t *testing.T) {
	root := repoRoot(t)

	servicePath := filepath.Join(root, "internal", "application", "service.go")
	data, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	content := string(data)

	// The marker for the old inline implementation: direct PRClients map
	// dereference. The delegating implementation builds a handler and calls
	// the handler, never touching s.PRClients[...] directly.
	if strings.Contains(content, "s.PRClients[") {
		t.Errorf("internal/application/service.go references s.PRClients[...] directly, " +
			"which is the marker of the duplicate PR-comment dispatch path. " +
			"Service.PRComment must delegate to PRCommentHandler.PRComment.")
	}
}

// TestAgentInvocationConstrainsPolicyWeakeningFields is a fitness function
// for system-intent Policy Authority: every field that can hide failing
// domains, skip verification, or shrink measurement must appear on
// agentInvocation so enforceAgentCapabilities can reject it.
func TestAgentInvocationConstrainsPolicyWeakeningFields(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "mcp", "capabilities.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse capabilities.go: %v", err)
	}
	required := []string{"TestArgs", "ConfigPath", "Domains", "CoverageScope", "FromProfile", "Incremental"}
	var fields []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "agentInvocation" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatal("agentInvocation is not a struct")
			}
			for _, field := range st.Fields.List {
				for _, n := range field.Names {
					fields = append(fields, n.Name)
				}
			}
		}
	}
	if len(fields) == 0 {
		t.Fatal("agentInvocation struct not found in capabilities.go")
	}
	have := map[string]bool{}
	for _, f := range fields {
		have[f] = true
	}
	for _, want := range required {
		if !have[want] {
			t.Errorf("agentInvocation missing %s; agent mode cannot enforce that policy-weakening input", want)
		}
	}
}

// TestAgentModeAdvertisesOnlyLoopTools keeps the agent surface at check,
// suggest, and debt. Expanding it without an explicit product decision
// regresses tool-selection reliability.
func TestAgentModeAdvertisesOnlyLoopTools(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal", "mcp", "server.go"))
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, `s.server.Tool("check")`) ||
		!strings.Contains(src, `s.server.Tool("suggest")`) ||
		!strings.Contains(src, `s.server.Tool("debt")`) {
		t.Error("agent-loop tools check/suggest/debt must stay registered")
	}
	if !strings.Contains(src, "if agent {\n\t\treturn") {
		t.Error("registerTools must return after the agent-loop tools so CI-only tools stay hidden")
	}
}
