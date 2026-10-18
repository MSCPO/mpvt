package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mpvt/internal/config"
)

func TestLocalConfigCycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpvt_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	cfg := config.LocalConfig{
		ProjectId:    "proj-12345",
		ProjectSlug:  "my-app",
		ProjectTitle: "My Application",
		Endpoint:     "http://localhost:3002",
	}

	if err := config.SaveLocalConfig(tempDir, cfg); err != nil {
		t.Fatalf("failed to save local config: %v", err)
	}

	// Change working directory to nested subDir and verify FindLocalConfig finds it
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	loaded, dir, err := config.FindLocalConfig()
	if err != nil {
		t.Fatalf("FindLocalConfig failed: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected to find local config, got nil")
	}

	if loaded.ProjectId != "proj-12345" {
		t.Errorf("expected ProjectId proj-12345, got %s", loaded.ProjectId)
	}
	if loaded.ProjectSlug != "my-app" {
		t.Errorf("expected ProjectSlug my-app, got %s", loaded.ProjectSlug)
	}
	if dir != filepath.Join(tempDir, config.LocalDirName) {
		t.Errorf("expected dir %s, got %s", filepath.Join(tempDir, config.LocalDirName), dir)
	}
}

func TestEnsureProjectBound(t *testing.T) {
	c := &config.Config{}
	_, err := c.EnsureProjectBound()
	if err == nil {
		t.Fatal("expected error when not bound, got nil")
	}
	if !errors.Is(err, config.ErrProjectNotBound) {
		t.Errorf("expected ErrProjectNotBound, got %v", err)
	}

	c.Local = &config.LocalConfig{
		ProjectId: "proj-1",
	}
	bound, err := c.EnsureProjectBound()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bound.ProjectId != "proj-1" {
		t.Errorf("expected proj-1, got %s", bound.ProjectId)
	}
}

func TestFindLocalConfig_StopsAtGitRoot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpvt_git_boundary_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save a config in the parent directory outside the git repo
	parentCfg := config.LocalConfig{
		ProjectId:   "parent-malicious-id",
		ProjectSlug: "parent-slug",
	}
	if err := config.SaveLocalConfig(tempDir, parentCfg); err != nil {
		t.Fatalf("failed to save parent local config: %v", err)
	}

	// Create a nested git repo inside tempDir
	repoDir := filepath.Join(tempDir, "myrepo")
	gitDir := filepath.Join(repoDir, ".git")
	subDir := filepath.Join(repoDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("failed to chdir to subDir: %v", err)
	}

	// Should NOT inherit parentCfg because traversal must stop at repoDir (.git)
	loaded, _, err := config.FindLocalConfig()
	if err != nil {
		t.Fatalf("FindLocalConfig failed: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil config due to .git boundary, but found: %+v", loaded)
	}
}

func TestFindLocalConfig_StopsAtUserHome(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpvt_home_boundary_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save a config in parent directory
	parentCfg := config.LocalConfig{
		ProjectId:   "parent-ambient-id",
		ProjectSlug: "parent-ambient-slug",
	}
	if err := config.SaveLocalConfig(tempDir, parentCfg); err != nil {
		t.Fatalf("failed to save parent local config: %v", err)
	}

	// Mock HOME as a subfolder
	mockHome := filepath.Join(tempDir, "mockhome")
	workDir := filepath.Join(mockHome, "workspace")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create workDir: %v", err)
	}

	t.Setenv("HOME", mockHome)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("failed to chdir to workDir: %v", err)
	}

	loaded, _, err := config.FindLocalConfig()
	if err != nil {
		t.Fatalf("FindLocalConfig failed: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil config due to HOME boundary, but found: %+v", loaded)
	}
}

func TestGetEffectiveEndpoint_Precedence(t *testing.T) {
	// 1. Default when both are empty
	c1 := &config.Config{}
	if ep := c1.GetEffectiveEndpoint(); ep != config.DefaultEndpoint {
		t.Errorf("expected default endpoint %s, got %s", config.DefaultEndpoint, ep)
	}

	// 2. Untrusted local config endpoint must NOT override default/global endpoint (SEC-01)
	c2 := &config.Config{
		Local: &config.LocalConfig{
			Endpoint: "https://malicious-site.com/api",
		},
	}
	if ep := c2.GetEffectiveEndpoint(); ep != config.DefaultEndpoint {
		t.Errorf("expected local config endpoint to be ignored and default returned, got %s", ep)
	}

	// 3. Explicit global/flag endpoint is respected
	c3 := &config.Config{
		Global: config.GlobalConfig{
			Endpoint: "https://custom.api.mscpo.org",
		},
		Local: &config.LocalConfig{
			Endpoint: "https://malicious-site.com/api",
		},
	}
	if ep := c3.GetEffectiveEndpoint(); ep != "https://custom.api.mscpo.org" {
		t.Errorf("expected global endpoint to prevail, got %s", ep)
	}
}

func TestGlobalConfigPermissions(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "mpvt_home_perm_*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempHome, ".config"))

	path, err := config.GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath failed: %v", err)
	}

	dir := filepath.Dir(path)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat config dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected dir permissions 0700, got %o", dirInfo.Mode().Perm())
	}

	cfg := config.GlobalConfig{
		Token: "super-secret-token",
	}
	if err := config.SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("expected file permissions 0600, got %o", fileInfo.Mode().Perm())
	}
}
