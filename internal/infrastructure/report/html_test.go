package report

import (
	"bytes"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestWriteHTML(t *testing.T) {
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
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Fatal("expected HTML doctype")
	}
	if !strings.Contains(output, "core") {
		t.Fatal("expected domain name in output")
	}
	if !strings.Contains(output, "83.2%") {
		t.Fatal("expected coverage percentage")
	}
}

func TestWriteHTMLWithFiles(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "core",
			Percent:  85.0,
			Required: 80,
			Status:   domain.StatusPass,
		}},
		Files: []domain.FileResult{{
			File:     "internal/core/a.go",
			Percent:  90.0,
			Required: 85,
			Status:   domain.StatusPass,
		}},
	}
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "internal/core/a.go") {
		t.Fatal("expected file path in output")
	}
}

func TestWriteHTMLWithWarnings(t *testing.T) {
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
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "shared directory") {
		t.Fatal("expected warning in output")
	}
}

func TestWriteHTMLFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: false,
		Domains: []domain.DomainResult{{
			Domain:   "core",
			Percent:  70.0,
			Required: 80,
			Status:   domain.StatusFail,
		}},
	}
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "FAIL") {
		t.Fatal("expected FAIL status in output")
	}
}

func TestWriteHTMLEmptyDomains(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed:  true,
		Domains: []domain.DomainResult{},
	}
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Fatal("expected HTML doctype even for empty domains")
	}
}

func TestWriteHTMLMultipleDomains(t *testing.T) {
	buf := new(bytes.Buffer)
	res := domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 95.0, Required: 80, Status: domain.StatusPass},
			{Domain: "api", Percent: 85.0, Required: 80, Status: domain.StatusPass},
			{Domain: "cli", Percent: 82.0, Required: 80, Status: domain.StatusPass},
		},
	}
	if err := (Writer{}).Write(buf, res, application.OutputHTML); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "core") {
		t.Fatal("expected core domain")
	}
	if !strings.Contains(output, "api") {
		t.Fatal("expected api domain")
	}
	if !strings.Contains(output, "cli") {
		t.Fatal("expected cli domain")
	}
}
