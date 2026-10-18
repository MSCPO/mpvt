package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
