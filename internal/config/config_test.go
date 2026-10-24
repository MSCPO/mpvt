// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mpvt/internal/config"
)

func isolateTestHome(t *testing.T, tmpHome string) {
	t.Helper()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("AppData", filepath.Join(tmpHome, "AppData", "Roaming"))
	t.Setenv("LocalAppData", filepath.Join(tmpHome, "AppData", "Local"))
}

func TestLocalConfigCycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpvt_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	if realTempDir, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = realTempDir
	}

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
	realDir, _ := filepath.EvalSymlinks(dir)
	realExpected, _ := filepath.EvalSymlinks(filepath.Join(tempDir, config.LocalDirName))
	if realDir != realExpected {
		t.Errorf("expected dir %s, got %s", realExpected, realDir)
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
	if realTempDir, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = realTempDir
	}

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
	if realTempDir, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = realTempDir
	}

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

	isolateTestHome(t, mockHome)

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
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	path, err := config.GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath failed: %v", err)
	}

	cfg := config.GlobalConfig{
		Token: "super-secret-token",
	}
	if err := config.SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	if runtime.GOOS == "windows" {
		return // Windows does not support POSIX permission bits (0700/0600)
	}

	dir := filepath.Dir(path)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat config dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected dir permissions 0700, got %o", dirInfo.Mode().Perm())
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("expected file permissions 0600, got %o", fileInfo.Mode().Perm())
	}
}

func TestGetGlobalConfigPath_NoSideEffects(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	path, err := config.GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath failed: %v", err)
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected config dir to not exist before save, but found it at %s", dir)
	}
}

func TestLoadGlobalConfig_EnvOverrides(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	t.Setenv("MPVT_ENDPOINT", "https://env-api.example.com")
	t.Setenv("MPVT_AUTH_ENDPOINT", "https://env-auth.example.com")
	t.Setenv("MPVT_TOKEN", "env-token-123")
	t.Setenv("MPVT_INTERNAL_TOKEN", "env-internal-456")
	t.Setenv("MPVT_USER_ID", "env-user-789")
	t.Setenv("MPVT_USER_EMAIL", "env-user@example.com")
	t.Setenv("MPVT_USER_ROLE", "ADMIN")

	// 1. Without any existing config file
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if cfg.Endpoint != "https://env-api.example.com" {
		t.Errorf("expected endpoint https://env-api.example.com, got %s", cfg.Endpoint)
	}
	if cfg.AuthEndpoint != "https://env-auth.example.com" {
		t.Errorf("expected auth endpoint https://env-auth.example.com, got %s", cfg.AuthEndpoint)
	}
	if cfg.Token != "env-token-123" {
		t.Errorf("expected token env-token-123, got %s", cfg.Token)
	}
	if cfg.InternalToken != "env-internal-456" {
		t.Errorf("expected internal token env-internal-456, got %s", cfg.InternalToken)
	}
	if cfg.UserId != "env-user-789" {
		t.Errorf("expected user id env-user-789, got %s", cfg.UserId)
	}
	if cfg.UserEmail != "env-user@example.com" {
		t.Errorf("expected email env-user@example.com, got %s", cfg.UserEmail)
	}
	if cfg.UserRole != "ADMIN" {
		t.Errorf("expected role ADMIN, got %s", cfg.UserRole)
	}

	// 2. With an existing config file whose values should be overridden
	savedCfg := config.GlobalConfig{
		Endpoint: "https://file-api.example.com",
		Token:    "file-token",
	}
	if err := config.SaveGlobalConfig(savedCfg); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	cfgOverridden, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}
	if cfgOverridden.Endpoint != "https://env-api.example.com" {
		t.Errorf("expected overridden endpoint, got %s", cfgOverridden.Endpoint)
	}
	if cfgOverridden.Token != "env-token-123" {
		t.Errorf("expected overridden token, got %s", cfgOverridden.Token)
	}
}

func TestEnsureGlobalConfigDir(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	dir, err := config.EnsureGlobalConfigDir()
	if err != nil {
		t.Fatalf("EnsureGlobalConfigDir failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat created config dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
		t.Errorf("expected 0700 perms, got %o", info.Mode().Perm())
	}
}

func TestLoadActiveConfig_WithAndWithoutLocal(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	workDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	// 1. Without local config
	cfgWithout, err := config.LoadActiveConfig()
	if err != nil {
		t.Fatalf("LoadActiveConfig without local failed: %v", err)
	}
	if cfgWithout.Local != nil {
		t.Errorf("expected nil local config, got %+v", cfgWithout.Local)
	}

	// 2. With local config
	local := config.LocalConfig{
		ProjectId:   "proj-active-123",
		ProjectSlug: "active-slug",
	}
	if err := config.SaveLocalConfig(workDir, local); err != nil {
		t.Fatalf("SaveLocalConfig failed: %v", err)
	}

	cfgWith, err := config.LoadActiveConfig()
	if err != nil {
		t.Fatalf("LoadActiveConfig with local failed: %v", err)
	}
	if cfgWith.Local == nil || cfgWith.Local.ProjectId != "proj-active-123" {
		t.Errorf("unexpected local config: %+v", cfgWith.Local)
	}
}

func TestEnsureProjectBoundAndEffectiveEndpoint(t *testing.T) {
	// 1. Nil local
	cfgNil := &config.Config{}
	if _, err := cfgNil.EnsureProjectBound(); !errors.Is(err, config.ErrProjectNotBound) {
		t.Errorf("expected ErrProjectNotBound for nil local, got %v", err)
	}

	// 2. Empty ProjectId
	cfgEmpty := &config.Config{
		Local: &config.LocalConfig{ProjectId: ""},
	}
	if _, err := cfgEmpty.EnsureProjectBound(); !errors.Is(err, config.ErrProjectNotBound) {
		t.Errorf("expected ErrProjectNotBound for empty ProjectId, got %v", err)
	}

	// 3. Valid ProjectId
	cfgValid := &config.Config{
		Local: &config.LocalConfig{ProjectId: "valid-p1"},
	}
	bound, err := cfgValid.EnsureProjectBound()
	if err != nil || bound.ProjectId != "valid-p1" {
		t.Errorf("expected valid local config, got %v, err: %v", bound, err)
	}

	// 4. GetEffectiveEndpoint default vs global
	cfgDefault := &config.Config{}
	if ep := cfgDefault.GetEffectiveEndpoint(); ep != config.DefaultEndpoint {
		t.Errorf("expected default endpoint %s, got %s", config.DefaultEndpoint, ep)
	}

	cfgCustom := &config.Config{
		Global: config.GlobalConfig{Endpoint: "https://custom.endpoint.com"},
	}
	if ep := cfgCustom.GetEffectiveEndpoint(); ep != "https://custom.endpoint.com" {
		t.Errorf("expected custom endpoint, got %s", ep)
	}
}

func TestLoadGlobalConfig_CorruptYAML(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	path, err := config.GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("GetGlobalConfigPath failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	// Write corrupt YAML
	if err := os.WriteFile(path, []byte(":: invalid: yaml: content: ["), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = config.LoadGlobalConfig()
	if err == nil {
		t.Errorf("expected error loading corrupt YAML, got nil")
	}

	_, err = config.LoadActiveConfig()
	if err == nil {
		t.Errorf("expected error loading active config when global is corrupt, got nil")
	}
}

func TestConfigHome_NoEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("AppData", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	_, err := config.EnsureGlobalConfigDir()
	if err == nil {
		t.Errorf("expected error for EnsureGlobalConfigDir with empty HOME/XDG/AppData, got nil")
	}

	_, err = config.GetGlobalConfigPath()
	if err == nil {
		t.Errorf("expected error for GetGlobalConfigPath with empty HOME/XDG/AppData, got nil")
	}
}

func TestSaveLocalConfig_InvalidPath(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Attempt to save into a subpath of a regular file
	err := config.SaveLocalConfig(filepath.Join(tmpFile, "invalid-dir"), config.LocalConfig{ProjectId: "p1"})
	if err == nil {
		t.Errorf("expected error saving local config to subpath of file, got nil")
	}
}
