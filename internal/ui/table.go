// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) ||
		r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1f64f) ||
		(r >= 0x1f900 && r <= 0x1f9ff)
}

func visibleWidth(s string) int {
	clean := stripANSI(s)
	width := 0
	for _, r := range clean {
		if isWideRune(r) {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

type Table struct {
	headers []string
	rows    [][]string
}

// NewTable creates a clean CLI table with ANSI-aware column alignment
func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
	}
}

// Append adds a row of columns
func (t *Table) Append(cols ...string) {
	t.rows = append(t.rows, cols)
}

// Render prints the formatted table to stdout
func (t *Table) Render() {
	t.RenderTo(os.Stdout)
}

// RenderTo outputs the formatted table to any io.Writer with ANSI-aware alignment
func (t *Table) RenderTo(w io.Writer) {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return
	}

	numCols := len(t.headers)
	for _, row := range t.rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	colWidths := make([]int, numCols)
	for i, h := range t.headers {
		w := visibleWidth(h)
		if w > colWidths[i] {
			colWidths[i] = w
		}
	}

	for _, row := range t.rows {
		for i, col := range row {
			w := visibleWidth(col)
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	const colPadding = 3

	// Render headers
	if len(t.headers) > 0 {
		for i, h := range t.headers {
			fmt.Fprint(w, Bold(h))
			if i < len(t.headers)-1 {
				pad := colWidths[i] - visibleWidth(h) + colPadding
				if pad > 0 {
					fmt.Fprint(w, strings.Repeat(" ", pad))
				}
			}
		}
		fmt.Fprintln(w)
	}

	// Render rows
	for _, row := range t.rows {
		for i, col := range row {
			fmt.Fprint(w, col)
			if i < len(row)-1 {
				pad := colWidths[i] - visibleWidth(col) + colPadding
				if pad > 0 {
					fmt.Fprint(w, strings.Repeat(" ", pad))
				}
			}
		}
		fmt.Fprintln(w)
	}
}
