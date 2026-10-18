package ui_test

import (
	"strings"
	"testing"

	"mpvt/internal/ui"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		id     string
		maxLen int
		want   string
	}{
		{"abcdefghij", 8, "abcdefgh"},
		{"abcdefgh", 8, "abcdefgh"},
		{"abc", 8, "abc"},
		{"", 8, ""},
		{"1234567890", 4, "1234"},
		{"12345", 0, ""},
		{"12345", -1, ""},
	}

	for _, tt := range tests {
		got := ui.ShortID(tt.id, tt.maxLen)
		if got != tt.want {
			t.Errorf("ShortID(%q, %d) = %q; want %q", tt.id, tt.maxLen, got, tt.want)
		}
	}
}

func TestPromptSelectEmpty(t *testing.T) {
	idx, val, err := ui.PromptSelect("Choose", nil)
	if err == nil {
		t.Fatalf("expected error for empty items, got nil (idx=%d, val=%q)", idx, val)
	}
	if err.Error() != "cannot select from an empty list" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPromptTextWithInjectedReader(t *testing.T) {
	input := "custom value\n"
	ui.SetReader(strings.NewReader(input))
	defer ui.SetReader(nil)

	val, err := ui.PromptText("Test", "default")
	if err != nil {
		t.Fatalf("PromptText failed: %v", err)
	}
	if val != "custom value" {
		t.Errorf("expected 'custom value', got %q", val)
	}
}

func TestPromptConfirm(t *testing.T) {
	ui.SetReader(strings.NewReader("y\n"))
	defer ui.SetReader(nil)
	if !ui.PromptConfirm("Proceed?", false) {
		t.Errorf("expected true for 'y', got false")
	}

	ui.SetReader(strings.NewReader("n\n"))
	if ui.PromptConfirm("Proceed?", true) {
		t.Errorf("expected false for 'n', got true")
	}

	ui.SetReader(strings.NewReader("\n"))
	if !ui.PromptConfirm("Proceed?", true) {
		t.Errorf("expected default true for empty input, got false")
	}
}

func TestPromptSelectWithInjectedReader(t *testing.T) {
	ui.SetReader(strings.NewReader("2\n"))
	defer ui.SetReader(nil)

	idx, val, err := ui.PromptSelect("Pick", []string{"First", "Second", "Third"})
	if err != nil {
		t.Fatalf("PromptSelect failed: %v", err)
	}
	if idx != 1 || val != "Second" {
		t.Errorf("expected idx=1, val='Second', got idx=%d, val=%q", idx, val)
	}
}

