# Technical Design Document: Language-Agnostic Coverage Enforcement

## Overview

This document describes the technical design for extending coverctl to support multiple programming languages while maintaining the existing DDD architecture, testability, and idiomatic Go patterns.

**Goal:** Add multi-language support through modular parsers and runners without breaking existing Go functionality.

---

## Architecture

### Current Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                             │
│                    internal/cli/                             │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
│                internal/application/                         │
│    ┌─────────────────────────────────────────────────────┐  │
│    │                    Service                           │  │
│    │  - Check, Report, Run, Detect, Badge, Trend, etc.   │  │
│    └─────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     Domain Layer                             │
│                  internal/domain/                            │
│         Policy, CoverageStat, Result, History                │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│               internal/infrastructure/                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ coverprofile │  │    gotool    │  │  autodetect  │       │
│  │   (Go only)  │  │   (Go only)  │  │   (Go only)  │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                             │
│                    internal/cli/                             │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
│                internal/application/                         │
│    ┌─────────────────────────────────────────────────────┐  │
│    │                    Service                           │  │
│    │     Uses ProfileParser, CoverageRunner interfaces    │  │
│    └─────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     Domain Layer                             │
│                  internal/domain/                            │
│       Policy, CoverageStat, Result, History (unchanged)     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│               internal/infrastructure/                       │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    parsers/                             │ │
│  │  ┌────────┐  ┌────────┐  ┌───────────┐  ┌───────────┐ │ │
│  │  │  go/   │  │ lcov/  │  │cobertura/ │  │  jacoco/  │ │ │
│  │  └────────┘  └────────┘  └───────────┘  └───────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    runners/                             │ │
│  │  ┌────────┐  ┌─────────┐  ┌────────┐  ┌────────────┐  │ │
│  │  │  go/   │  │ python/ │  │ node/  │  │   rust/    │  │ │
│  │  └────────┘  └─────────┘  └────────┘  └────────────┘  │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    detect/                              │ │
│  │              Language & Format Detection                │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

### New Structure

```
internal/
├── application/
│   ├── service.go          # Updated to use new interfaces
│   └── types.go            # Extended with Language, Format types
├── domain/                 # UNCHANGED
│   ├── policy.go
│   └── history.go
├── infrastructure/
│   ├── parsers/
│   │   ├── parser.go       # Factory function
│   │   ├── parser_test.go
│   │   ├── go/
│   │   │   ├── parser.go   # Moved from coverprofile/
│   │   │   └── parser_test.go
│   │   ├── lcov/
│   │   │   ├── parser.go
│   │   │   └── parser_test.go
│   │   ├── cobertura/
│   │   │   ├── parser.go
│   │   │   └── parser_test.go
│   │   └── jacoco/
│   │       ├── parser.go
│   │       └── parser_test.go
│   ├── runners/
│   │   ├── runner.go       # Factory function
│   │   ├── runner_test.go
│   │   ├── go/
│   │   │   ├── runner.go   # Moved from gotool/
│   │   │   └── runner_test.go
│   │   ├── python/
│   │   │   ├── runner.go
│   │   │   └── runner_test.go
│   │   ├── node/
│   │   │   ├── runner.go
│   │   │   └── runner_test.go
│   │   └── rust/
│   │       ├── runner.go
│   │       └── runner_test.go
│   ├── detect/
│   │   ├── language.go     # Project language detection
│   │   ├── language_test.go
│   │   ├── format.go       # Coverage format detection
│   │   └── format_test.go
│   ├── gotool/             # DEPRECATED - redirect to runners/go
│   ├── coverprofile/       # DEPRECATED - redirect to parsers/go
│   └── ... (other unchanged)
```

---

## Interface Design

### Core Interfaces

```go
// internal/application/types.go

// Language represents a programming language.
type Language string

const (
    LanguageAuto       Language = "auto"
    LanguageGo         Language = "go"
    LanguagePython     Language = "python"
    LanguageTypeScript Language = "typescript"
    LanguageJavaScript Language = "javascript"
    LanguageJava       Language = "java"
    LanguageRust       Language = "rust"
)

// Format represents a coverage profile format.
type Format string

const (
    FormatAuto      Format = "auto"
    FormatGo        Format = "go"
    FormatLCOV      Format = "lcov"
    FormatCobertura Format = "cobertura"
    FormatJaCoCo    Format = "jacoco"
    FormatLLVMCov   Format = "llvm-cov"
)

// ProfileParser parses coverage profiles into domain stats.
// Implementations exist for each supported format.
type ProfileParser interface {
    // Parse reads a coverage profile and returns file-level stats.
    Parse(path string) (map[string]domain.CoverageStat, error)

    // ParseAll merges multiple profiles into unified stats.
    ParseAll(paths []string) (map[string]domain.CoverageStat, error)

    // Format returns the format this parser handles.
    Format() Format
}

// CoverageRunner executes tests and generates coverage profiles.
// Implementations exist for each supported language.
type CoverageRunner interface {
    // Run executes tests with coverage and returns the profile path.
    Run(ctx context.Context, opts RunOptions) (string, error)

    // RunIntegration runs integration tests with coverage.
    RunIntegration(ctx context.Context, opts IntegrationOptions) (string, error)

    // Language returns the language this runner handles.
    Language() Language

    // OutputFormat returns the coverage format produced by this runner.
    OutputFormat() Format
}

// LanguageDetector detects project language from filesystem markers.
type LanguageDetector interface {
    // Detect returns the detected language for the given directory.
    Detect(dir string) (Language, error)

    // Markers returns the files/patterns used for detection.
    Markers() []string
}

// FormatDetector detects coverage format from file content.
type FormatDetector interface {
    // Detect returns the format of the given coverage file.
    Detect(path string) (Format, error)
}
```

### Extended Config

```go
// internal/application/types.go

// Config represents validated, application-ready configuration.
type Config struct {
    Version     int
    Language    Language       // NEW: Project language
    Profile     ProfileConfig  // NEW: Profile configuration
    Policy      domain.Policy
    Exclude     []string
    Files       []domain.FileRule
    Diff        DiffConfig
    Merge       MergeConfig
    Integration IntegrationConfig
    Annotations AnnotationsConfig
}

// ProfileConfig configures coverage profile handling.
type ProfileConfig struct {
    Format Format // Coverage format (auto, go, lcov, cobertura, jacoco)
    Path   string // Default profile path
}
```

---

## Implementation Details

### Parser Factory

```go
// internal/infrastructure/parsers/parser.go

package parsers

import (
    "fmt"

    "go.klarlabs.de/coverctl/internal/application"
    "go.klarlabs.de/coverctl/internal/infrastructure/parsers/cobertura"
    goparser "go.klarlabs.de/coverctl/internal/infrastructure/parsers/go"
    "go.klarlabs.de/coverctl/internal/infrastructure/parsers/jacoco"
    "go.klarlabs.de/coverctl/internal/infrastructure/parsers/lcov"
)

// New creates a parser for the specified format.
func New(format application.Format) (application.ProfileParser, error) {
    switch format {
    case application.FormatGo:
        return goparser.New(), nil
    case application.FormatLCOV:
        return lcov.New(), nil
    case application.FormatCobertura:
        return cobertura.New(), nil
    case application.FormatJaCoCo:
        return jacoco.New(), nil
    case application.FormatAuto:
        return &autoParser{}, nil
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }
}

// autoParser detects format and delegates to appropriate parser.
type autoParser struct{}

func (p *autoParser) Parse(path string) (map[string]domain.CoverageStat, error) {
    format, err := detect.Format(path)
    if err != nil {
        return nil, err
    }
    parser, err := New(format)
    if err != nil {
        return nil, err
    }
    return parser.Parse(path)
}

func (p *autoParser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
    if len(paths) == 0 {
        return nil, fmt.Errorf("no profiles provided")
    }
    // All profiles must be same format
    format, err := detect.Format(paths[0])
    if err != nil {
        return nil, err
    }
    parser, err := New(format)
    if err != nil {
        return nil, err
    }
    return parser.ParseAll(paths)
}

func (p *autoParser) Format() application.Format {
    return application.FormatAuto
}
```

### LCOV Parser

```go
// internal/infrastructure/parsers/lcov/parser.go

package lcov

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"

    "go.klarlabs.de/coverctl/internal/application"
    "go.klarlabs.de/coverctl/internal/domain"
)

// Parser implements ProfileParser for LCOV format.
type Parser struct{}

func New() *Parser {
    return &Parser{}
}

func (p *Parser) Format() application.Format {
    return application.FormatLCOV
}

func (p *Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("open lcov file: %w", err)
    }
    defer file.Close()

    stats := make(map[string]domain.CoverageStat)
    scanner := bufio.NewScanner(file)

    var currentFile string
    var covered, total int

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        switch {
        case strings.HasPrefix(line, "SF:"):
            // Source file
            currentFile = strings.TrimPrefix(line, "SF:")
            covered, total = 0, 0

        case strings.HasPrefix(line, "DA:"):
            // Data line: DA:line_number,execution_count
            parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
            if len(parts) >= 2 {
                total++
                count, _ := strconv.Atoi(parts[1])
                if count > 0 {
                    covered++
                }
            }

        case strings.HasPrefix(line, "LF:"):
            // Lines found (total lines)
            lf, _ := strconv.Atoi(strings.TrimPrefix(line, "LF:"))
            if lf > total {
                total = lf
            }

        case strings.HasPrefix(line, "LH:"):
            // Lines hit (covered lines)
            lh, _ := strconv.Atoi(strings.TrimPrefix(line, "LH:"))
            if lh > covered {
                covered = lh
            }

        case line == "end_of_record":
            // End of file record
            if currentFile != "" {
                stats[currentFile] = domain.CoverageStat{
                    Covered: covered,
                    Total:   total,
                }
            }
            currentFile = ""
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("scan lcov file: %w", err)
    }

    return stats, nil
}

func (p *Parser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
    merged := make(map[string]domain.CoverageStat)

    for _, path := range paths {
        stats, err := p.Parse(path)
        if err != nil {
            return nil, err
        }
        for file, stat := range stats {
            existing := merged[file]
            existing.Total = max(existing.Total, stat.Total)
            existing.Covered = max(existing.Covered, stat.Covered)
            merged[file] = existing
        }
    }

    return merged, nil
}
```

### Cobertura XML Parser

```go
// internal/infrastructure/parsers/cobertura/parser.go

package cobertura

import (
    "encoding/xml"
    "fmt"
    "os"

    "go.klarlabs.de/coverctl/internal/application"
    "go.klarlabs.de/coverctl/internal/domain"
)

// coverage represents the root Cobertura XML element.
type coverage struct {
    XMLName  xml.Name  `xml:"coverage"`
    Packages []pkg     `xml:"packages>package"`
}

type pkg struct {
    Name    string  `xml:"name,attr"`
    Classes []class `xml:"classes>class"`
}

type class struct {
    Name     string `xml:"name,attr"`
    Filename string `xml:"filename,attr"`
    Lines    []line `xml:"lines>line"`
}

type line struct {
    Number int `xml:"number,attr"`
    Hits   int `xml:"hits,attr"`
}

// Parser implements ProfileParser for Cobertura XML format.
type Parser struct{}

func New() *Parser {
    return &Parser{}
}

func (p *Parser) Format() application.Format {
    return application.FormatCobertura
}

func (p *Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("open cobertura file: %w", err)
    }
    defer file.Close()

    var cov coverage
    if err := xml.NewDecoder(file).Decode(&cov); err != nil {
        return nil, fmt.Errorf("decode cobertura xml: %w", err)
    }

    stats := make(map[string]domain.CoverageStat)

    for _, pkg := range cov.Packages {
        for _, cls := range pkg.Classes {
            filename := cls.Filename
            if filename == "" {
                continue
            }

            var covered, total int
            for _, ln := range cls.Lines {
                total++
                if ln.Hits > 0 {
                    covered++
                }
            }

            existing := stats[filename]
            existing.Total += total
            existing.Covered += covered
            stats[filename] = existing
        }
    }

    return stats, nil
}

func (p *Parser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
    merged := make(map[string]domain.CoverageStat)

    for _, path := range paths {
        stats, err := p.Parse(path)
        if err != nil {
            return nil, err
        }
        for file, stat := range stats {
            existing := merged[file]
            existing.Total += stat.Total
            existing.Covered += stat.Covered
            merged[file] = existing
        }
    }

    return merged, nil
}
```

### Format Detection

```go
// internal/infrastructure/detect/format.go

package detect

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"

    "go.klarlabs.de/coverctl/internal/application"
)

// Format detects the coverage format of the given file.
func Format(path string) (application.Format, error) {
    // Check extension first
    ext := filepath.Ext(path)
    switch ext {
    case ".out":
        return application.FormatGo, nil
    case ".info", ".lcov":
        return application.FormatLCOV, nil
    }

    // Sniff file content
    file, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for i := 0; i < 10 && scanner.Scan(); i++ {
        line := scanner.Text()

        // Go coverage format
        if strings.HasPrefix(line, "mode:") {
            return application.FormatGo, nil
        }

        // LCOV format
        if strings.HasPrefix(line, "TN:") || strings.HasPrefix(line, "SF:") {
            return application.FormatLCOV, nil
        }

        // XML formats
        if strings.Contains(line, "<?xml") || strings.Contains(line, "<coverage") {
            // Need to peek further to distinguish Cobertura vs JaCoCo
            return detectXMLFormat(path)
        }
    }

    return "", fmt.Errorf("unable to detect coverage format for %s", path)
}

func detectXMLFormat(path string) (application.Format, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }

    content := string(data)

    // JaCoCo has <report> root or jacoco-specific attributes
    if strings.Contains(content, "<report ") || strings.Contains(content, "jacoco") {
        return application.FormatJaCoCo, nil
    }

    // Cobertura has <coverage> root
    if strings.Contains(content, "<coverage ") {
        return application.FormatCobertura, nil
    }

    return "", fmt.Errorf("unable to detect XML coverage format")
}
```

### Language Detection

```go
// internal/infrastructure/detect/language.go

package detect

import (
    "os"
    "path/filepath"

    "go.klarlabs.de/coverctl/internal/application"
)

// Marker represents a file marker for language detection.
type Marker struct {
    File     string
    Language application.Language
    Priority int // Higher = more specific
}

var markers = []Marker{
    // Go
    {"go.mod", application.LanguageGo, 10},

    // Python
    {"pyproject.toml", application.LanguagePython, 10},
    {"setup.py", application.LanguagePython, 8},
    {"requirements.txt", application.LanguagePython, 5},
    {"Pipfile", application.LanguagePython, 8},

    // TypeScript/JavaScript
    {"tsconfig.json", application.LanguageTypeScript, 10},
    {"package.json", application.LanguageJavaScript, 5}, // Could be TS too

    // Java
    {"pom.xml", application.LanguageJava, 10},
    {"build.gradle", application.LanguageJava, 10},
    {"build.gradle.kts", application.LanguageJava, 10},

    // Rust
    {"Cargo.toml", application.LanguageRust, 10},
}

// Language detects the primary language of the project at dir.
func Language(dir string) (application.Language, error) {
    var best application.Language
    var bestPriority int

    for _, m := range markers {
        path := filepath.Join(dir, m.File)
        if _, err := os.Stat(path); err == nil {
            if m.Priority > bestPriority {
                best = m.Language
                bestPriority = m.Priority
            }
        }
    }

    if best == "" {
        return "", fmt.Errorf("unable to detect project language in %s", dir)
    }

    return best, nil
}

// HasMarker checks if a specific marker file exists.
func HasMarker(dir, file string) bool {
    path := filepath.Join(dir, file)
    _, err := os.Stat(path)
    return err == nil
}
```

### Python Runner (Example)

```go
// internal/infrastructure/runners/python/runner.go

package python

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"

    "go.klarlabs.de/coverctl/internal/application"
)

// Runner implements CoverageRunner for Python projects.
type Runner struct {
    Exec func(ctx context.Context, dir string, args []string) error
}

func New() *Runner {
    return &Runner{
        Exec: runCommand,
    }
}

func (r *Runner) Language() application.Language {
    return application.LanguagePython
}

func (r *Runner) OutputFormat() application.Format {
    return application.FormatLCOV // pytest-cov can output LCOV
}

func (r *Runner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
    profile := opts.ProfilePath
    if profile == "" {
        profile = ".cover/coverage.info"
    }

    // Ensure output directory exists
    if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
        return "", err
    }

    // Build pytest command with coverage
    args := []string{
        "-m", "pytest",
        "--cov=.",
        "--cov-report=lcov:" + profile,
    }

    // Add test patterns if specified
    if opts.BuildFlags.Run != "" {
        args = append(args, "-k", opts.BuildFlags.Run)
    }

    // Add verbose flag
    if opts.BuildFlags.Verbose {
        args = append(args, "-v")
    }

    // Execute pytest
    if err := r.Exec(ctx, ".", append([]string{"python"}, args...)); err != nil {
        return "", fmt.Errorf("pytest failed: %w", err)
    }

    return profile, nil
}

func (r *Runner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
    // Python doesn't have the same integration test pattern as Go
    // Just run with integration markers
    runOpts := application.RunOptions{
        ProfilePath: opts.Profile,
        BuildFlags: application.BuildFlags{
            Run: "integration", // pytest -k integration
        },
    }
    return r.Run(ctx, runOpts)
}

func runCommand(ctx context.Context, dir string, args []string) error {
    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    cmd.Dir = dir
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### Runner Factory

```go
// internal/infrastructure/runners/runner.go

package runners

import (
    "fmt"

    "go.klarlabs.de/coverctl/internal/application"
    gorunner "go.klarlabs.de/coverctl/internal/infrastructure/runners/go"
    "go.klarlabs.de/coverctl/internal/infrastructure/runners/node"
    "go.klarlabs.de/coverctl/internal/infrastructure/runners/python"
    "go.klarlabs.de/coverctl/internal/infrastructure/runners/rust"
)

// New creates a runner for the specified language.
func New(lang application.Language) (application.CoverageRunner, error) {
    switch lang {
    case application.LanguageGo:
        return gorunner.New(), nil
    case application.LanguagePython:
        return python.New(), nil
    case application.LanguageTypeScript, application.LanguageJavaScript:
        return node.New(), nil
    case application.LanguageRust:
        return rust.New(), nil
    default:
        return nil, fmt.Errorf("unsupported language: %s", lang)
    }
}

// ForLanguage is an alias for New for clarity.
func ForLanguage(lang application.Language) (application.CoverageRunner, error) {
    return New(lang)
}
```

---

## Configuration Schema (v2)

### YAML Schema Extension

```yaml
# .coverctl.yaml - Version 2 with language support

version: 2

# Language configuration (optional, auto-detected if omitted)
language: auto  # auto | go | python | typescript | javascript | java | rust

# Profile configuration (optional)
profile:
  format: auto  # auto | go | lcov | cobertura | jacoco | llvm-cov
  path: .cover/coverage.out  # Default profile path

# Policy (unchanged from v1)
policy:
  default:
    min: 75
  domains:
    - name: core
      match: ["./internal/core/..."]  # Go-style
      min: 85
    - name: core
      match: ["src/core/**/*.py"]     # Glob-style for other languages
      min: 85

# Exclude patterns (unchanged)
exclude:
  - "**/generated/**"
  - "**/mocks/**"
  - "**/*_test.go"
  - "**/*.test.ts"

# Other options (unchanged)
diff:
  enabled: true
  base: origin/main

merge:
  profiles:
    - .cover/unit.out
    - .cover/integration.out
```

### JSON Schema Update

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://coverctl.dev/schema/coverctl.v2.json",
  "title": "coverctl configuration",
  "type": "object",
  "properties": {
    "version": {
      "type": "integer",
      "enum": [1, 2],
      "description": "Configuration version"
    },
    "language": {
      "type": "string",
      "enum": ["auto", "go", "python", "typescript", "javascript", "java", "rust"],
      "default": "auto",
      "description": "Project language"
    },
    "profile": {
      "type": "object",
      "properties": {
        "format": {
          "type": "string",
          "enum": ["auto", "go", "lcov", "cobertura", "jacoco", "llvm-cov"],
          "default": "auto"
        },
        "path": {
          "type": "string",
          "default": ".cover/coverage.out"
        }
      }
    }
  }
}
```

---

## Migration Strategy

### Backward Compatibility

1. **Version 1 configs** continue to work unchanged
2. **Go is the default** when language not specified
3. **Existing parsers** moved but re-exported from original location

```go
// internal/infrastructure/coverprofile/parser.go
// DEPRECATED: Use internal/infrastructure/parsers/go instead

package coverprofile

import goparser "go.klarlabs.de/coverctl/internal/infrastructure/parsers/go"

// Parser is deprecated. Use parsers.New(application.FormatGo) instead.
type Parser = goparser.Parser
```

### Feature Flags (Optional)

```go
// For gradual rollout
var (
    EnableMultiLanguage = os.Getenv("COVERCTL_MULTI_LANGUAGE") == "1"
)
```

---

## Testing Strategy

### Unit Tests (TDD)

```go
// internal/infrastructure/parsers/lcov/parser_test.go

func TestParser_Parse_ValidLCOV(t *testing.T) {
    content := `TN:
SF:src/main.py
DA:1,1
DA:2,1
DA:3,0
LF:3
LH:2
end_of_record`

    tmpfile := createTempFile(t, content)
    defer os.Remove(tmpfile)

    parser := New()
    stats, err := parser.Parse(tmpfile)

    require.NoError(t, err)
    require.Len(t, stats, 1)
    assert.Equal(t, 2, stats["src/main.py"].Covered)
    assert.Equal(t, 3, stats["src/main.py"].Total)
}

func TestParser_Parse_MultipleFiles(t *testing.T) {
    content := `SF:src/a.py
DA:1,1
LF:1
LH:1
end_of_record
SF:src/b.py
DA:1,0
DA:2,0
LF:2
LH:0
end_of_record`

    tmpfile := createTempFile(t, content)
    defer os.Remove(tmpfile)

    parser := New()
    stats, err := parser.Parse(tmpfile)

    require.NoError(t, err)
    require.Len(t, stats, 2)
    assert.Equal(t, 1, stats["src/a.py"].Covered)
    assert.Equal(t, 0, stats["src/b.py"].Covered)
}
```

### Integration Tests

```go
// internal/infrastructure/parsers/lcov/integration_test.go

func TestParser_RealWorldFiles(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    testCases := []struct {
        name     string
        file     string
        minFiles int
    }{
        {"pytest-cov output", "testdata/pytest-cov.info", 10},
        {"nyc output", "testdata/nyc.info", 5},
        {"c8 output", "testdata/c8.info", 3},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            parser := New()
            stats, err := parser.Parse(tc.file)

            require.NoError(t, err)
            assert.GreaterOrEqual(t, len(stats), tc.minFiles)
        })
    }
}
```

### Format Detection Tests

```go
// internal/infrastructure/detect/format_test.go

func TestFormat_DetectsGoFromModeHeader(t *testing.T) {
    content := "mode: atomic\nfoo.go:1.1,2.2 1 1"
    tmpfile := createTempFile(t, content)

    format, err := Format(tmpfile)

    require.NoError(t, err)
    assert.Equal(t, application.FormatGo, format)
}

func TestFormat_DetectsLCOVFromSFHeader(t *testing.T) {
    content := "SF:main.py\nDA:1,1\nend_of_record"
    tmpfile := createTempFile(t, content)

    format, err := Format(tmpfile)

    require.NoError(t, err)
    assert.Equal(t, application.FormatLCOV, format)
}

func TestFormat_DetectsCoberturaFromXML(t *testing.T) {
    content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages><package name="test"/></packages>
</coverage>`
    tmpfile := createTempFile(t, content)

    format, err := Format(tmpfile)

    require.NoError(t, err)
    assert.Equal(t, application.FormatCobertura, format)
}
```

---

## Implementation Phases

### Phase 1: Profile Parsers (Week 1)

| Day | Task |
|-----|------|
| 1 | Create `parsers/` directory structure, move Go parser |
| 2 | Implement LCOV parser with tests |
| 3 | Implement Cobertura parser with tests |
| 4 | Implement format auto-detection |
| 5 | Integrate with Service, update CLI |

### Phase 2: Language Detection (Week 1-2)

| Day | Task |
|-----|------|
| 6 | Implement language detection |
| 7 | Extend config schema to v2 |
| 8 | Update autodetect for multi-language |
| 9 | Integration testing |
| 10 | Documentation |

### Phase 3: Runners (Week 2-3)

| Day | Task |
|-----|------|
| 11 | Create `runners/` structure, move Go runner |
| 12 | Implement Python runner |
| 13 | Implement Node.js runner |
| 14 | Implement Rust runner |
| 15 | Integration testing |

### Phase 4: Plugin (Week 4)

| Day | Task |
|-----|------|
| 16 | Create plugin manifest |
| 17 | Create slash commands |
| 18 | Create skills |
| 19 | Testing & documentation |
| 20 | Marketplace submission |

---

## Open Questions

1. **Domain Pattern Syntax:** Should we use Go-style `./pkg/...` or glob `pkg/**/*.go`?
   - **Recommendation:** Support both, detect based on language

2. **JaCoCo vs Cobertura:** Both are XML but different schemas. Merge into one parser?
   - **Recommendation:** Keep separate for accuracy

3. **Runner Dependencies:** Should runners check for tool availability?
   - **Recommendation:** Yes, with helpful error messages

4. **Monorepo Support:** How to handle multi-language monorepos?
   - **Recommendation:** Per-directory language detection, or explicit config

---

## Dependencies

```go
// go.mod additions

require (
    // Existing
    go.klarlabs.de/mcp v1.4.0

    // New (if needed)
    // None - using stdlib xml and text parsing
)
```

---

## Appendix

### LCOV Format Reference

```
TN:<test name>
SF:<source file path>
FN:<line number>,<function name>
FNDA:<execution count>,<function name>
FNF:<number of functions found>
FNH:<number of functions hit>
BRDA:<line number>,<block number>,<branch number>,<taken>
BRF:<number of branches found>
BRH:<number of branches hit>
DA:<line number>,<execution count>[,<checksum>]
LF:<number of lines found>
LH:<number of lines hit>
end_of_record
```

### Cobertura XML Reference

```xml
<?xml version="1.0"?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage line-rate="0.85" branch-rate="0.75" version="1.0">
  <packages>
    <package name="com.example" line-rate="0.90">
      <classes>
        <class name="MyClass" filename="src/MyClass.java" line-rate="0.90">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>
```
