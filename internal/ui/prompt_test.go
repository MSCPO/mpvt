package ui_test

import (
	"bytes"
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
		{"项目生命周期", 4, "项目生命"},
		{"🚀🌟🔥✨🎉", 3, "🚀🌟🔥"},
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

func TestPromptMultilineWithInjectedReader(t *testing.T) {
	// Case 1: Multiple paragraphs separated by single empty line, finished by two empty lines
	input := "Paragraph 1\n\nParagraph 2\n\n\n"
	ui.SetReader(strings.NewReader(input))
	defer ui.SetReader(nil)

	got, err := ui.PromptMultiline("Enter description")
	if err != nil {
		t.Fatalf("PromptMultiline failed: %v", err)
	}
	expected := "Paragraph 1\n\nParagraph 2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	// Case 2: Immediate empty input returns empty string
	ui.SetReader(strings.NewReader("\n"))
	gotEmpty, err := ui.PromptMultiline("Enter description")
	if err != nil {
		t.Fatalf("PromptMultiline failed on empty input: %v", err)
	}
	if gotEmpty != "" {
		t.Errorf("expected empty string, got %q", gotEmpty)
	}
}

func TestBadges(t *testing.T) {
	// Status badge tests
	statusCases := []string{"IN_DEVELOPMENT", "PUBLISHED", "PLANNING", "ARCHIVED", "UNKNOWN"}
	for _, sc := range statusCases {
		s := ui.FormatStatusBadge(sc)
		if !strings.Contains(s, sc) {
			t.Errorf("expected status badge for %s to contain %s, got: %s", sc, sc, s)
		}
	}

	// Milestone badge tests
	msCases := []string{"COMPLETED", "IN_PROGRESS", "PLANNED", "CUSTOM"}
	for _, mc := range msCases {
		m := ui.FormatMilestoneBadge(mc)
		if !strings.Contains(m, mc) {
			t.Errorf("expected milestone badge for %s to contain %s, got: %s", mc, mc, m)
		}
	}

	// Proposal badge tests
	propCases := []string{"MERGED", "OPEN", "CLOSED", "UNKNOWN"}
	for _, pc := range propCases {
		p := ui.FormatProposalBadge(pc)
		if !strings.Contains(p, pc) {
			t.Errorf("expected proposal badge for %s to contain %s, got: %s", pc, pc, p)
		}
	}
}

func TestStyleHelpers(t *testing.T) {
	// Call each helper function to verify formatting and execution without panics
	ui.Success("operation successful")
	ui.Successf("created %d items", 3)
	ui.Error("operation failed")
	ui.Errorf("failed with code %d", 500)
	ui.Warning("caution required")
	ui.Warningf("warning on line %d", 42)
	ui.Info("informational note")
	ui.Infof("connected to %s", "localhost")
}

func TestTableRender_ANSIAlignment(t *testing.T) {
	tbl := ui.NewTable("Column1", "Status", "Description")
	tbl.Append("plain", ui.FormatStatusBadge("IN_DEVELOPMENT"), "Some details")
	tbl.Append(ui.Cyan("colored"), ui.FormatStatusBadge("PUBLISHED"), "More details")

	var buf bytes.Buffer
	tbl.RenderTo(&buf)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (1 header + 2 rows), got %d: %q", len(lines), output)
	}

	// Verify headers are present
	if !strings.Contains(lines[0], "Column1") || !strings.Contains(lines[0], "Status") {
		t.Errorf("header line missing expected columns: %s", lines[0])
	}

	// Test Render to stdout directly
	tbl.Render()
}

func TestPromptSelect_RetryOnInvalid(t *testing.T) {
	// First invalid string, then 0 (out of bounds), then 5 (out of bounds), then 2 (valid)
	ui.SetReader(strings.NewReader("invalid\n0\n5\n2\n"))
	defer ui.SetReader(nil)

	idx, val, err := ui.PromptSelect("Pick", []string{"First", "Second", "Third"})
	if err != nil {
		t.Fatalf("PromptSelect failed: %v", err)
	}
	if idx != 1 || val != "Second" {
		t.Errorf("expected idx=1, val='Second', got idx=%d, val=%q", idx, val)
	}
}

func TestPromptText_DefaultValues(t *testing.T) {
	// 1. Enter key pressed (empty input) returns defaultValue
	ui.SetReader(strings.NewReader("\n"))
	defer ui.SetReader(nil)
	val, err := ui.PromptText("Label", "fallback")
	if err != nil || val != "fallback" {
		t.Errorf("expected 'fallback', got %q, err: %v", val, err)
	}

	// 2. Immediate EOF with defaultValue
	ui.SetReader(strings.NewReader(""))
	val, err = ui.PromptText("Label", "fallback-eof")
	if err != nil || val != "fallback-eof" {
		t.Errorf("expected 'fallback-eof', got %q, err: %v", val, err)
	}

	// 3. Immediate EOF without defaultValue
	ui.SetReader(strings.NewReader(""))
	_, err = ui.PromptText("Label", "")
	if err == nil {
		t.Errorf("expected error on EOF without default, got nil")
	}
}

func TestPromptMultiline_EOFWithLines(t *testing.T) {
	ui.SetReader(strings.NewReader("Line 1\nLine 2"))
	defer ui.SetReader(nil)

	got, err := ui.PromptMultiline("Label")
	if err != nil {
		t.Fatalf("PromptMultiline failed: %v", err)
	}
	if got != "Line 1\nLine 2" {
		t.Errorf("expected 'Line 1\\nLine 2', got %q", got)
	}
}

func TestPromptConfirm_Variations(t *testing.T) {
	// Test "yes"
	ui.SetReader(strings.NewReader("yes\n"))
	defer ui.SetReader(nil)
	if !ui.PromptConfirm("Confirm?", false) {
		t.Errorf("expected true for 'yes'")
	}

	// Test default false when empty
	ui.SetReader(strings.NewReader("\n"))
	if ui.PromptConfirm("Confirm?", false) {
		t.Errorf("expected false for default false")
	}

	// Test EOF
	ui.SetReader(strings.NewReader(""))
	if ui.PromptConfirm("Confirm?", true) {
		t.Errorf("expected false on EOF")
	}
}

func TestTableRender_EmptyAndWide(t *testing.T) {
	// Empty table
	emptyTbl := ui.NewTable()
	var bufEmpty bytes.Buffer
	emptyTbl.RenderTo(&bufEmpty)
	if bufEmpty.Len() != 0 {
		t.Errorf("expected empty output for empty table, got %q", bufEmpty.String())
	}

	// Wide CJK and Emoji columns
	wideTbl := ui.NewTable("项目名称", "状态", "备注")
	wideTbl.Append("MSCPO 平台", "🚀 运行中", "包含特殊全角字符（测试）")
	var bufWide bytes.Buffer
	wideTbl.RenderTo(&bufWide)
	if !strings.Contains(bufWide.String(), "MSCPO 平台") {
		t.Errorf("expected wide table to contain CJK text, got: %s", bufWide.String())
	}
}
