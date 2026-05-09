package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSurveyAnswer_AcceptsAllForms(t *testing.T) {
	cases := []struct {
		in       string
		wantCode string
	}{
		{"v", "very-disappointed"},
		{"VERY", "very-disappointed"},
		{"Very Disappointed", "very-disappointed"},
		{"s", "somewhat-disappointed"},
		{"somewhat", "somewhat-disappointed"},
		{"n", "not-disappointed"},
		{"not disappointed", "not-disappointed"},
		{"skip", "skip"},
		{"", "skip"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := normalizeSurveyAnswer(c.in)
			if !ok {
				t.Fatalf("expected ok for %q", c.in)
			}
			if got != c.wantCode {
				t.Errorf("normalizeSurveyAnswer(%q) = %q; want %q", c.in, got, c.wantCode)
			}
		})
	}
}

func TestNormalizeSurveyAnswer_RejectsGarbage(t *testing.T) {
	cases := []string{"yes", "no", "maybe", "👍"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, ok := normalizeSurveyAnswer(c); ok {
				t.Errorf("normalizeSurveyAnswer(%q) should reject", c)
			}
		})
	}
}

func TestPromptSurvey_ReturnsLine(t *testing.T) {
	in := strings.NewReader("very\n")
	var out bytes.Buffer
	got, err := promptSurvey(in, &out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "very" {
		t.Errorf("got %q; want %q", got, "very")
	}
	if !strings.Contains(out.String(), "How would you feel") {
		t.Error("expected prompt header in output")
	}
}

func TestPromptSurvey_EOFReturnsSkip(t *testing.T) {
	in := strings.NewReader("")
	var out bytes.Buffer
	got, err := promptSurvey(in, &out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "skip" {
		t.Errorf("EOF should map to skip, got %q", got)
	}
}

func TestWriteSurveyResponse_AppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	if err := writeSurveyResponse(dir, "very-disappointed"); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := writeSurveyResponse(dir, "somewhat-disappointed"); err != nil {
		t.Fatalf("write 2: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "survey.jsonl"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), data)
	}
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d not valid JSON: %v", i, err)
		}
		if m["event"] != "pmf_survey" {
			t.Errorf("line %d wrong event: %v", i, m["event"])
		}
		if m["answer"] == "" {
			t.Errorf("line %d missing answer", i)
		}
		if _, ok := m["ts"].(string); !ok {
			t.Errorf("line %d missing ts", i)
		}
	}
}

func TestRunSurvey_NonInteractiveSkip(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runSurvey(context.Background(),
		[]string{"--answer=skip", "--data-dir=" + dir},
		&stdout, &stderr, GlobalOptions{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	// Skip must NOT write to survey.jsonl.
	if _, err := os.Stat(filepath.Join(dir, "survey.jsonl")); !os.IsNotExist(err) {
		t.Errorf("skip should not create survey.jsonl: %v", err)
	}
}

func TestRunSurvey_NonInteractiveValid(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runSurvey(context.Background(),
		[]string{"--answer=very", "--data-dir=" + dir},
		&stdout, &stderr, GlobalOptions{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "survey.jsonl"))
	if err != nil {
		t.Fatalf("survey.jsonl not written: %v", err)
	}
	if !strings.Contains(string(data), `"answer":"very-disappointed"`) {
		t.Errorf("expected very-disappointed in: %s", data)
	}
}

func TestRunSurvey_RejectsBadAnswer(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runSurvey(context.Background(),
		[]string{"--answer=maybe", "--data-dir=" + dir},
		&stdout, &stderr, GlobalOptions{})
	if code != 2 {
		t.Errorf("expected exit 2 for bad answer, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unrecognized") {
		t.Errorf("expected unrecognized error in: %s", stderr.String())
	}
}
