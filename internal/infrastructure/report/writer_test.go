package report

import (
	"bytes"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestWriteText(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "core",
			Percent:  83.2,
			Required: 80,
			Status:   domain.StatusPass,
		}},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "core") {
		t.Fatalf("expected domain in output")
	}
}

func TestWriteJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{Passed: false}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "\"pass\": false") {
		t.Fatalf("expected JSON summary")
	}
}

func TestWriteWarningsText(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "core",
			Percent:  90,
			Required: 80,
			Status:   domain.StatusPass,
		}},
		Warnings: []string{"shared directory used by core and api"},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "Warnings:") {
		t.Fatalf("expected warnings section")
	}
}

func TestWriteWarningsJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed:   true,
		Warnings: []string{"shared directory used by core and api"},
	}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "\"warnings\"") {
		t.Fatalf("expected warnings field")
	}
}

func TestWriteFileRulesText(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Files: []domain.FileResult{{
			File:     "internal/core/a.go",
			Percent:  88.8,
			Required: 90,
			Status:   domain.StatusFail,
		}},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "File rules:") {
		t.Fatalf("expected file rules section")
	}
}

func TestWriteFileRulesJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Files: []domain.FileResult{{
			File:     "internal/core/a.go",
			Percent:  88.8,
			Required: 90,
			Status:   domain.StatusFail,
		}},
	}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "\"files\"") {
		t.Fatalf("expected files field")
	}
}

func TestWriteUnsupportedFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{Passed: true}
	err := (Writer{}).Write(buf, res, application.OutputFormat("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("expected unsupported format error, got: %v", err)
	}
}

func TestWriteEmptyFormat(t *testing.T) {
	// Empty format should default to text
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "core",
			Percent:  85.0,
			Required: 80,
			Status:   domain.StatusPass,
		}},
	}
	if err := (Writer{}).Write(buf, res, ""); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Should output text format (contains domain name without JSON structure)
	if !strings.Contains(buf.String(), "core") {
		t.Fatalf("expected domain in text output")
	}
	if strings.Contains(buf.String(), "{") {
		t.Fatalf("expected text output, not JSON")
	}
}

func TestWriteEmptyDomains(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed:  true,
		Domains: []domain.DomainResult{},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Should still have header
	if !strings.Contains(buf.String(), "Domain") {
		t.Fatalf("expected header in output")
	}
}

func TestWriteEmptyDomainsJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed:  true,
		Domains: []domain.DomainResult{},
	}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Should have empty domains array
	if !strings.Contains(buf.String(), "\"domains\": []") {
		t.Fatalf("expected empty domains array in JSON")
	}
}

func TestWriteCombinedOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: false,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 85.0, Required: 80, Status: domain.StatusPass},
			{Domain: "api", Percent: 70.0, Required: 75, Status: domain.StatusFail},
		},
		Files: []domain.FileResult{
			{File: "core/main.go", Percent: 90.0, Required: 85, Status: domain.StatusPass},
			{File: "api/handler.go", Percent: 60.0, Required: 80, Status: domain.StatusFail},
		},
		Warnings: []string{
			"shared directory used by core and api",
			"domain 'utils' has no matched packages",
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	// Check domains
	if !strings.Contains(output, "core") {
		t.Fatal("expected core domain")
	}
	if !strings.Contains(output, "api") {
		t.Fatal("expected api domain")
	}
	// Check file rules section
	if !strings.Contains(output, "File rules:") {
		t.Fatal("expected File rules section")
	}
	if !strings.Contains(output, "core/main.go") {
		t.Fatal("expected core/main.go file")
	}
	// Check warnings section
	if !strings.Contains(output, "Warnings:") {
		t.Fatal("expected Warnings section")
	}
	if !strings.Contains(output, "shared directory") {
		t.Fatal("expected shared directory warning")
	}
	if !strings.Contains(output, "no matched packages") {
		t.Fatal("expected no matched packages warning")
	}
}

func TestWriteCombinedOutputJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: false,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 85.0, Required: 80, Status: domain.StatusPass},
		},
		Files: []domain.FileResult{
			{File: "core/main.go", Percent: 90.0, Required: 85, Status: domain.StatusPass},
		},
		Warnings: []string{"test warning"},
	}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	// Check all sections are present
	if !strings.Contains(output, "\"domains\"") {
		t.Fatal("expected domains field")
	}
	if !strings.Contains(output, "\"files\"") {
		t.Fatal("expected files field")
	}
	if !strings.Contains(output, "\"warnings\"") {
		t.Fatal("expected warnings field")
	}
	if !strings.Contains(output, "\"summary\"") {
		t.Fatal("expected summary field")
	}
	if !strings.Contains(output, "\"pass\": false") {
		t.Fatal("expected pass: false")
	}
}

func TestWriteMultipleDomainsText(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 95.0, Required: 80, Status: domain.StatusPass},
			{Domain: "api", Percent: 85.0, Required: 80, Status: domain.StatusPass},
			{Domain: "cli", Percent: 82.0, Required: 80, Status: domain.StatusPass},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	// All domains should be present
	if !strings.Contains(output, "core") {
		t.Fatal("expected core domain")
	}
	if !strings.Contains(output, "api") {
		t.Fatal("expected api domain")
	}
	if !strings.Contains(output, "cli") {
		t.Fatal("expected cli domain")
	}
	// Verify percentages are formatted correctly
	if !strings.Contains(output, "95.0%") {
		t.Fatal("expected 95.0%")
	}
}

func TestWriteFailStatus(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: false,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 70.0, Required: 80, Status: domain.StatusFail},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "FAIL") {
		t.Fatal("expected FAIL status in output")
	}
}

func TestWritePassStatus(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 90.0, Required: 80, Status: domain.StatusPass},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "PASS") {
		t.Fatal("expected PASS status in output")
	}
}

func TestWriteWarnStatus(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 85.0, Required: 80, Status: domain.StatusWarn},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputText); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "WARN") {
		t.Fatal("expected WARN status in output")
	}
}

func TestWriteWarnStatusJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 85.0, Required: 80, Status: domain.StatusWarn},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "WARN") {
		t.Fatal("expected WARN status in JSON output")
	}
}

func TestWriteBriefPass(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Covered: 850, Total: 1000, Percent: 85.0, Required: 80, Status: domain.StatusPass},
			{Domain: "api", Covered: 820, Total: 1000, Percent: 82.0, Required: 80, Status: domain.StatusPass},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputBrief); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	// Should be single line
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("expected single line output, got: %q", output)
	}
	if !strings.HasPrefix(output, "PASS") {
		t.Fatalf("expected PASS prefix, got: %q", output)
	}
	if !strings.Contains(output, "2/2 domains passing") {
		t.Fatalf("expected domain count, got: %q", output)
	}
	// Should not contain failing section
	if strings.Contains(output, "failing:") {
		t.Fatalf("unexpected failing section in passing result")
	}
}

func TestWriteBriefFail(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: false,
		Domains: []domain.DomainResult{
			{Domain: "core", Covered: 850, Total: 1000, Percent: 85.0, Required: 80, Status: domain.StatusPass},
			{Domain: "api", Covered: 650, Total: 1000, Percent: 65.0, Required: 80, Status: domain.StatusFail},
			{Domain: "cli", Covered: 700, Total: 1000, Percent: 70.0, Required: 80, Status: domain.StatusFail},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputBrief); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	// Should be single line
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("expected single line output, got: %q", output)
	}
	if !strings.HasPrefix(output, "FAIL") {
		t.Fatalf("expected FAIL prefix, got: %q", output)
	}
	if !strings.Contains(output, "1/3 domains passing") {
		t.Fatalf("expected domain count, got: %q", output)
	}
	if !strings.Contains(output, "failing:") {
		t.Fatalf("expected failing section, got: %q", output)
	}
	if !strings.Contains(output, "api (65.0%)") {
		t.Fatalf("expected api in failing list, got: %q", output)
	}
	if !strings.Contains(output, "cli (70.0%)") {
		t.Fatalf("expected cli in failing list, got: %q", output)
	}
}

func TestWriteBriefWithWarnings(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Covered: 850, Total: 1000, Percent: 85.0, Required: 80, Status: domain.StatusPass},
		},
		Warnings: []string{"warning1", "warning2"},
	}
	if err := (Writer{}).Write(buf, res, application.OutputBrief); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "2 warnings") {
		t.Fatalf("expected warnings count, got: %q", output)
	}
}

func TestWriteBriefEmpty(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed:  true,
		Domains: []domain.DomainResult{},
	}
	if err := (Writer{}).Write(buf, res, application.OutputBrief); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.HasPrefix(output, "PASS") {
		t.Fatalf("expected PASS for empty result, got: %q", output)
	}
	if !strings.Contains(output, "0/0 domains passing") {
		t.Fatalf("expected 0/0 domains, got: %q", output)
	}
}
