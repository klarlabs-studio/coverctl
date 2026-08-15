package eval

import (
	"context"
	"testing"
)

func TestScoreToolSelection(t *testing.T) {
	if err := ScoreToolSelection([]string{"check", "suggest"}, []string{"suggest", "check"}); err != nil {
		t.Fatalf("order-insensitive match failed: %v", err)
	}
	if err := ScoreToolSelection([]string{"debt"}, []string{"check"}); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestScriptedSelector(t *testing.T) {
	s := ScriptedSelector{Tools: []string{"check"}}
	got, err := s.Select(context.Background(), "anything", []string{"check", "suggest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "check" {
		t.Fatalf("got %v", got)
	}
}

func TestParseToolList(t *testing.T) {
	got, err := parseToolList("Sure: [\"check\", \"debt\"]", []string{"check", "suggest", "debt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "check" || got[1] != "debt" {
		t.Fatalf("got %v", got)
	}
}
