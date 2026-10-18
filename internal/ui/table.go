package ui

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

type Table struct {
	headers []string
	rows    [][]string
}

// NewTable creates a clean CLI table using Go's built-in tabwriter
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

// RenderTo outputs the table to any io.Writer
func (t *Table) RenderTo(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 3, ' ', 0)
	if len(t.headers) > 0 {
		for i, h := range t.headers {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, Bold(h))
		}
		fmt.Fprintln(tw)
	}

	for _, row := range t.rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, col)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}
