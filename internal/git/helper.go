package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommitInfo struct {
	Hash        string
	Subject     string
	Body        string
	AuthorName  string
	AuthorEmail string
}

type GitCommitInfo = CommitInfo

func gitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), 5*time.Second)
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		return context.WithTimeout(ctx, 5*time.Second)
	}
	return ctx, func() {}
}

// IsGitRepo checks if current directory is inside a git work tree
func IsGitRepo(ctx context.Context) bool {
	c, cancel := gitContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(c, "git", "rev-parse", "--is-inside-work-tree")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Run() == nil
}

// GetLastCommit extracts the most recent git commit information
func GetLastCommit(ctx context.Context) (*CommitInfo, error) {
	c, cancel := gitContext(ctx)
	defer cancel()

	// Format: %H%x00%s%x00%b%x00%an%x00%ae
	cmd := exec.CommandContext(c, "git", "log", "-1", "--pretty=format:%H%x00%s%x00%b%x00%an%x00%ae")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "does not have any commits") || strings.Contains(errMsg, "unknown revision") || strings.Contains(errMsg, "尚无任何提交") {
			return nil, nil
		}
		if errMsg != "" {
			return nil, fmt.Errorf("git log failed: %s (%w)", errMsg, err)
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	parts := strings.Split(stdout.String(), "\x00")
	if len(parts) < 2 {
		return nil, nil
	}

	info := &CommitInfo{
		Hash:    parts[0],
		Subject: strings.TrimSpace(parts[1]),
	}

	if len(parts) > 2 {
		info.Body = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		info.AuthorName = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		info.AuthorEmail = strings.TrimSpace(parts[4])
	}

	return info, nil
}

// GetRepoRoot returns the top-level directory of the current git repository
func GetRepoRoot(ctx context.Context) (string, error) {
	c, cancel := gitContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(c, "git", "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("git rev-parse --show-toplevel failed: %s (%w)", errMsg, err)
		}
		return "", fmt.Errorf("git rev-parse --show-toplevel failed: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// EnsureGitIgnore checks if .gitignore contains .mpvt/ and appends it if not
func EnsureGitIgnore(repoDir string) (bool, error) {
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new .gitignore
			err := os.WriteFile(gitignorePath, []byte(".mpvt/\n"), 0644)
			return true, err
		}
		return false, err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".mpvt" || trimmed == ".mpvt/" {
			return false, nil
		}
	}

	// Append
	if !strings.HasSuffix(content, "\n") && len(content) > 0 {
		content += "\n"
	}
	content += ".mpvt/\n"
	err = os.WriteFile(gitignorePath, []byte(content), 0644)
	return true, err
}
