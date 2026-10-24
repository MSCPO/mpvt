package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var (
	customReader io.Reader
	reader       = bufio.NewReader(os.Stdin)
)

// SetReader allows configuring or injecting a custom input reader (e.g. for testing)
func SetReader(r io.Reader) {
	customReader = r
	if r == nil {
		reader = bufio.NewReader(os.Stdin)
	} else {
		reader = bufio.NewReader(r)
	}
}

// IsInteractive checks whether the standard input is connected to an interactive terminal
func IsInteractive() bool {
	if customReader != nil {
		return true
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// PromptText prompts for a single line text input
func PromptText(label, defaultValue string) (string, error) {
	if !IsInteractive() {
		if defaultValue != "" {
			return defaultValue, nil
		}
		return "", fmt.Errorf("interactive prompt requested for %q in non-interactive environment; please supply the corresponding command-line flag", label)
	}

	if defaultValue != "" {
		fmt.Printf("%s %s [%s]: ", Cyan("?"), Bold(label), Dim(defaultValue))
	} else {
		fmt.Printf("%s %s: ", Cyan("?"), Bold(label))
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && defaultValue != "" {
			return defaultValue, nil
		}
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}
	return input, nil
}

// PromptMultiline prompts for multi-line text input until an empty line is entered twice
func PromptMultiline(label string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("interactive multiline prompt requested for %q in non-interactive environment; please supply the content via flags or file", label)
	}

	fmt.Printf("%s %s %s:\n", Cyan("?"), Bold(label), Dim("(Press Enter twice or leave blank to finish)"))

	var lines []string
	consecutiveEmpty := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimRight(line, "\r\n")
				if trimmed != "" {
					lines = append(lines, trimmed)
				}
				if len(lines) > 0 {
					break
				}
				return "", nil
			}
			return "", err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			consecutiveEmpty++
			if len(lines) == 0 {
				return "", nil
			}
			if consecutiveEmpty >= 2 {
				break
			}
			continue
		}

		for i := 0; i < consecutiveEmpty; i++ {
			lines = append(lines, "")
		}
		consecutiveEmpty = 0
		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\n"), nil
}

// PromptConfirm prompts for a yes/no question
func PromptConfirm(label string, defaultYes bool) bool {
	if !IsInteractive() {
		return false // In non-interactive CI/CD, never auto-confirm destructive actions without explicit flags
	}

	defaultStr := "Y/n"
	if !defaultYes {
		defaultStr = "y/N"
	}

	fmt.Printf("%s %s [%s]: ", Yellow("?"), Bold(label), Dim(defaultStr))
	input, err := reader.ReadString('\n')
	if err != nil {
		return false // Never return true on EOF error in headless runs
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

// PromptSelect prompts user to select an item from a list by index
func PromptSelect(label string, items []string) (int, string, error) {
	if len(items) == 0 {
		return -1, "", errors.New("cannot select from an empty list")
	}
	if !IsInteractive() {
		return -1, "", fmt.Errorf("interactive selection requested for %q in non-interactive environment; please specify the selection via CLI flags", label)
	}
	fmt.Printf("%s %s:\n", Cyan("?"), Bold(label))
	for i, item := range items {
		fmt.Printf("  %s %s\n", Cyan(fmt.Sprintf("[%d]", i+1)), item)
	}

	for {
		fmt.Printf("Select [1-%d]: ", len(items))
		input, err := reader.ReadString('\n')
		if err != nil {
			return -1, "", err
		}
		input = strings.TrimSpace(input)
		idx, err := strconv.Atoi(input)
		if err == nil && idx >= 1 && idx <= len(items) {
			return idx - 1, items[idx-1], nil
		}
		fmt.Printf("%s Please enter a number between 1 and %d\n", Red("!"), len(items))
	}
}
