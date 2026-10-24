// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

func TestRootWhoamiCommand(t *testing.T) {
	cmd := rootCmd
	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "whoami" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'whoami' command to be registered on rootCmd")
	}
}

func TestSilenceFlags(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Errorf("expected rootCmd.SilenceErrors to be true")
	}
	if !rootCmd.SilenceUsage {
		t.Errorf("expected rootCmd.SilenceUsage to be true")
	}
}

func TestExecuteContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// rootCmd.ExecuteContext with cancelled context
	err := rootCmd.ExecuteContext(ctx)
	// Some commands may return context.Canceled if context is checked
	_ = err
}

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

func TestExecuteSuccess(t *testing.T) {
	tmpHome := t.TempDir()
	isolateTestHome(t, tmpHome)
	rootCmd.SetArgs([]string{"version"})
	defer rootCmd.SetArgs([]string{})
	Execute()
}

func TestMilestoneUse_SavesToLocalPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpvt_cmd_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmpDir, ".mpvt", "config.json")
	localCfg := config.LocalConfig{
		ProjectId:             "proj-123",
		ProjectSlug:           "my-project",
		ProjectTitle:          "My Project",
		DefaultMilestoneId:    "old-ms-id",
		DefaultMilestoneTitle: "Old Milestone",
	}
	if err := config.SaveLocalConfig(tmpDir, localCfg); err != nil {
		t.Fatal(err)
	}

	// Verify initial file was written
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not found at %s: %v", cfgPath, err)
	}

	// Switch working directory to subDir
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	// Setup activeConfig pointing to tmpDir/.mpvt
	activeConfig = &config.Config{
		Local:     &localCfg,
		LocalPath: filepath.Join(tmpDir, ".mpvt"),
	}

	// Run milestoneUseCmd to unset default milestone ("none")
	if err := milestoneUseCmd.RunE(milestoneUseCmd, []string{"none"}); err != nil {
		t.Fatalf("milestone use failed: %v", err)
	}

	// Verify tmpDir/.mpvt/config.json was updated
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read updated config: %v", err)
	}
	var updated config.LocalConfig
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("failed to unmarshal updated config: %v", err)
	}
	if updated.DefaultMilestoneId != "" {
		t.Errorf("expected DefaultMilestoneId to be unset, got %q", updated.DefaultMilestoneId)
	}

	// Verify no .mpvt directory was created in subDir (cwd)
	subMpvt := filepath.Join(subDir, ".mpvt")
	if _, err := os.Stat(subMpvt); !os.IsNotExist(err) {
		t.Errorf("expected subDir/.mpvt to not exist, but found it at %s", subMpvt)
	}

	// Verify ShortID usage
	short := ui.ShortID("1234567890abcdef", 8)
	if short != "12345678" {
		t.Errorf("expected 12345678, got %s", short)
	}
}

func TestOutputJSON(t *testing.T) {
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	payload := map[string]string{"status": "ok"}
	_ = outputJSON(payload)

	w.Close()
	os.Stdout = origStdout
	buf.ReadFrom(r)

	if !bytes.Contains(buf.Bytes(), []byte(`"status": "ok"`)) {
		t.Errorf("expected json output to contain status: ok, got: %s", buf.String())
	}
}

func TestVersionCommand(t *testing.T) {
	flagJSON = true
	defer func() { flagJSON = false }()

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command in JSON mode failed: %v", err)
	}

	flagJSON = false
	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestOfflineCommand_SkipsRemoteProjectResolution(t *testing.T) {
	// Setup isolated home
	tmpHome := t.TempDir()
	isolateTestHome(t, tmpHome)

	flagProject = "non-existent-project-xyz"
	defer func() { flagProject = "" }()

	offlineCmds := []*cobra.Command{versionCmd, authLoginCmd, initCmd, unlinkCmd}
	for _, c := range offlineCmds {
		err := rootCmd.PersistentPreRunE(c, []string{})
		if err != nil {
			t.Errorf("expected command %s to skip remote project resolution, got error: %v", c.Name(), err)
		}
	}
}

func TestAuthLogin_TokenStdin(t *testing.T) {
	tmpHome := t.TempDir()
	isolateTestHome(t, tmpHome)

	// Pipe token into stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()
	os.Stdin = r

	go func() {
		defer w.Close()
		_, _ = w.Write([]byte("mock-bearer-token-via-stdin\n"))
	}()

	loginTokenStdin = true
	defer func() {
		loginTokenStdin = false
		loginToken = ""
	}()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{
			InternalToken: "stale-internal-token",
		},
	}

	if err := authLoginCmd.RunE(authLoginCmd, []string{}); err != nil {
		t.Fatalf("auth login with token-stdin failed: %v", err)
	}

	if activeConfig.Global.Token != "mock-bearer-token-via-stdin" {
		t.Errorf("expected token mock-bearer-token-via-stdin, got %s", activeConfig.Global.Token)
	}
	if activeConfig.Global.InternalToken != "" {
		t.Errorf("expected internal token to be cleared when bearer token is saved, got %s", activeConfig.Global.InternalToken)
	}
}

func TestAuthLogout(t *testing.T) {
	tmpHome := t.TempDir()
	isolateTestHome(t, tmpHome)

	activeConfig = &config.Config{
		Global: config.GlobalConfig{
			Token:         "token-to-clear",
			InternalToken: "internal-to-clear",
			UserId:        "user-to-clear",
		},
	}

	if err := authLogoutCmd.RunE(authLogoutCmd, []string{}); err != nil {
		t.Fatalf("auth logout failed: %v", err)
	}

	if activeConfig.Global.Token != "" || activeConfig.Global.InternalToken != "" || activeConfig.Global.UserId != "" {
		t.Errorf("expected credentials to be cleared, got %+v", activeConfig.Global)
	}
}

func TestGettersAndSetters(t *testing.T) {
	oldClient := GetApiClient()
	oldConfig := GetActiveConfig()
	defer func() {
		SetApiClient(oldClient)
		SetActiveConfig(oldConfig)
	}()

	mockCfg := &config.Config{
		Global: config.GlobalConfig{Endpoint: "https://test.mscpo.org"},
	}
	mockClient := api.NewClient(mockCfg)

	SetActiveConfig(mockCfg)
	if got := GetActiveConfig(); got != mockCfg {
		t.Errorf("expected mockCfg, got %v", got)
	}

	SetApiClient(mockClient)
	if got := GetApiClient(); got != mockClient {
		t.Errorf("expected mockClient, got %v", got)
	}
}

func TestOutputJSON_Error(t *testing.T) {
	// Channels cannot be marshaled to JSON
	err := outputJSON(make(chan int))
	if err == nil {
		t.Errorf("expected error when marshaling unsupported channel type, got nil")
	}
}
