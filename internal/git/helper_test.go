package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mpvt/internal/git"
)

func TestEnsureGitIgnore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Case 1: .gitignore doesn't exist yet
	added, err := git.EnsureGitIgnore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Errorf("expected added=true when .gitignore did not exist")
	}

	content, err := os.ReadFile(filepath.Join(tempDir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(content), ".mpvt/") {
		t.Errorf("expected .gitignore to contain .mpvt/, got %s", string(content))
	}

	// Case 2: .gitignore already contains .mpvt/
	addedAgain, err := git.EnsureGitIgnore(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addedAgain {
		t.Errorf("expected added=false when .gitignore already contained .mpvt/")
	}
}

func setupTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test Committer")
	runGit("config", "user.email", "committer@example.com")
	runGit("commit", "--allow-empty", "-m", "Initial test commit\n\nCommit body detail")

	return dir
}

func TestGitHelpersInRepo(t *testing.T) {
	repoDir := setupTempGitRepo(t)
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(origWd)

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed to chdir to repoDir: %v", err)
	}

	ctx := context.Background()

	// 1. IsGitRepo
	if !git.IsGitRepo(ctx) {
		t.Errorf("expected IsGitRepo to return true in git repo")
	}

	// 2. GetRepoRoot
	root, err := git.GetRepoRoot(ctx)
	if err != nil {
		t.Fatalf("GetRepoRoot failed: %v", err)
	}
	evalRoot, _ := filepath.EvalSymlinks(root)
	evalRepoDir, _ := filepath.EvalSymlinks(repoDir)
	if evalRoot != evalRepoDir {
		t.Errorf("expected root %s, got %s", evalRepoDir, evalRoot)
	}

	// 3. GetLastCommit
	commit, err := git.GetLastCommit(ctx)
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}
	if commit == nil {
		t.Fatalf("expected commit info, got nil")
	}
	if len(commit.Hash) < 40 {
		t.Errorf("expected full 40-char hash, got %s", commit.Hash)
	}
	if commit.Subject != "Initial test commit" {
		t.Errorf("expected subject 'Initial test commit', got %s", commit.Subject)
	}
	if commit.Body != "Commit body detail" {
		t.Errorf("expected body 'Commit body detail', got %s", commit.Body)
	}
	if commit.AuthorName != "Test Committer" {
		t.Errorf("expected author 'Test Committer', got %s", commit.AuthorName)
	}
	if commit.AuthorEmail != "committer@example.com" {
		t.Errorf("expected email 'committer@example.com', got %s", commit.AuthorEmail)
	}
}

func TestGitHelpersInNonRepo(t *testing.T) {
	nonRepoDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(origWd)

	if err := os.Chdir(nonRepoDir); err != nil {
		t.Fatalf("failed to chdir to nonRepoDir: %v", err)
	}

	ctx := context.Background()

	// 1. IsGitRepo returns false
	if git.IsGitRepo(ctx) {
		t.Errorf("expected IsGitRepo to return false in non-git directory")
	}

	// 2. GetRepoRoot returns error with stderr content
	_, err = git.GetRepoRoot(ctx)
	if err == nil {
		t.Errorf("expected error from GetRepoRoot in non-git directory, got nil")
	}

	// 3. GetLastCommit returns error with stderr content
	_, err = git.GetLastCommit(ctx)
	if err == nil {
		t.Errorf("expected error from GetLastCommit in non-git directory, got nil")
	}
}

func TestGitContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := git.GetLastCommit(ctx)
	if err == nil {
		t.Errorf("expected error when context is cancelled, got nil")
	}
}
