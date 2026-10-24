package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	Green   = color.New(color.FgGreen).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Yellow  = color.New(color.FgYellow).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
	Magenta = color.New(color.FgMagenta).SprintFunc()
	Blue    = color.New(color.FgBlue).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Dim     = color.New(color.Faint).SprintFunc()
)

func Success(msg string) {
	fmt.Printf("%s %s\n", Green("✔"), msg)
}

func Successf(format string, a ...any) {
	fmt.Printf("%s %s\n", Green("✔"), fmt.Sprintf(format, a...))
}

func Error(msg string) {
	fmt.Printf("%s %s\n", Red("✘"), Red(msg))
}

func Errorf(format string, a ...any) {
	fmt.Printf("%s %s\n", Red("✘"), Red(fmt.Sprintf(format, a...)))
}

func Warning(msg string) {
	fmt.Printf("%s %s\n", Yellow("!"), Yellow(msg))
}

func Warningf(format string, a ...any) {
	fmt.Printf("%s %s\n", Yellow("!"), Yellow(fmt.Sprintf(format, a...)))
}

func Info(msg string) {
	fmt.Printf("%s %s\n", Cyan("ℹ"), msg)
}

func Infof(format string, a ...any) {
	fmt.Printf("%s %s\n", Cyan("ℹ"), fmt.Sprintf(format, a...))
}

// FormatStatusBadge renders project status with icon and color
func FormatStatusBadge(status string) string {
	switch strings.ToUpper(status) {
	case "IN_DEVELOPMENT":
		return color.New(color.FgGreen, color.Bold).Sprint("🛠️  IN_DEVELOPMENT")
	case "PUBLISHED":
		return color.New(color.FgCyan, color.Bold).Sprint("🚀 PUBLISHED")
	case "PLANNING":
		return color.New(color.FgYellow, color.Bold).Sprint("💡 PLANNING")
	case "ARCHIVED":
		return color.New(color.FgHiBlack, color.Bold).Sprint("📦 ARCHIVED")
	default:
		return status
	}
}

// FormatMilestoneBadge renders milestone status
func FormatMilestoneBadge(status string) string {
	switch strings.ToUpper(status) {
	case "COMPLETED":
		return color.New(color.FgGreen).Sprint("[COMPLETED]")
	case "IN_PROGRESS":
		return color.New(color.FgCyan).Sprint("[IN_PROGRESS]")
	case "PLANNED":
		return color.New(color.FgYellow).Sprint("[PLANNED]")
	default:
		return "[" + status + "]"
	}
}

// FormatProposalBadge renders proposal status
func FormatProposalBadge(status string) string {
	switch strings.ToUpper(status) {
	case "MERGED":
		return color.New(color.FgMagenta, color.Bold).Sprint("MERGED")
	case "OPEN":
		return color.New(color.FgGreen, color.Bold).Sprint("OPEN")
	case "CLOSED":
		return color.New(color.FgRed, color.Bold).Sprint("CLOSED")
	default:
		return status
	}
}

// ShortID returns safely truncated ID up to maxLen runes (UTF-8 safe)
func ShortID(id string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(id)
	if len(runes) <= maxLen {
		return id
	}
	return string(runes[:maxLen])
}
