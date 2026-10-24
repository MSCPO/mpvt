// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultEndpoint = "https://api.mscpo.com/api/projectvitals"
	ConfigDirName   = "mpvt"
	LocalDirName    = ".mpvt"
	LocalConfigFile = "config.json"
)

// GlobalConfig stores global CLI preferences and credentials in ~/.config/mpvt/config.yaml
type GlobalConfig struct {
	Endpoint      string `yaml:"endpoint"`
	AuthEndpoint  string `yaml:"auth_endpoint,omitempty"`
	Token         string `yaml:"token,omitempty"`
	RefreshToken  string `yaml:"refresh_token,omitempty"`
	InternalToken string `yaml:"internal_token,omitempty"`
	UserId        string `yaml:"user_id,omitempty"`
	UserRole      string `yaml:"user_role,omitempty"`
	UserEmail     string `yaml:"user_email,omitempty"`
	UserName      string `yaml:"user_name,omitempty"`
}

// LocalConfig stores project binding information in ./.mpvt/config.json
type LocalConfig struct {
	ProjectId             string `json:"project_id"`
	ProjectSlug           string `json:"project_slug"`
	ProjectTitle          string `json:"project_title"`
	Endpoint              string `json:"endpoint,omitempty"`
	DefaultMilestoneId    string `json:"default_milestone_id,omitempty"`
	DefaultMilestoneTitle string `json:"default_milestone_title,omitempty"`
}

// Config is the merged active configuration
type Config struct {
	Global    GlobalConfig
	Local     *LocalConfig
	LocalPath string // Path to .mpvt directory if found
}

// EnsureGlobalConfigDir ensures ~/.config/mpvt directory exists with 0700 permissions
func EnsureGlobalConfigDir() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configHome, ConfigDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to set permissions on config directory %s: %w", dir, err)
	}
	return dir, nil
}

// GetGlobalConfigPath returns ~/.config/mpvt/config.yaml without mutative side effects
func GetGlobalConfigPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configHome, ConfigDirName)
	return filepath.Join(dir, "config.yaml"), nil
}

// isSameDir checks if two directory paths resolve to the same underlying directory on disk
func isSameDir(dir1, dir2 string) bool {
	if dir1 == dir2 {
		return true
	}
	fi1, err1 := os.Stat(dir1)
	fi2, err2 := os.Stat(dir2)
	if err1 == nil && err2 == nil {
		return os.SameFile(fi1, fi2)
	}
	return false
}

// LoadGlobalConfig reads ~/.config/mpvt/config.yaml
func LoadGlobalConfig() (GlobalConfig, error) {
	var cfg GlobalConfig
	cfg.Endpoint = DefaultEndpoint

	path, err := GetGlobalConfigPath()
	if err == nil {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("failed to parse global config: %w", err)
			}
		} else if !os.IsNotExist(readErr) {
			return cfg, fmt.Errorf("failed to read global config file %s: %w", path, readErr)
		}
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}

	// Environment variable overrides
	if envEndpoint := os.Getenv("MPVT_ENDPOINT"); envEndpoint != "" {
		cfg.Endpoint = envEndpoint
	}
	if envAuthEndpoint := os.Getenv("MPVT_AUTH_ENDPOINT"); envAuthEndpoint != "" {
		cfg.AuthEndpoint = envAuthEndpoint
	}
	if envToken := os.Getenv("MPVT_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}
	if envInternalToken := os.Getenv("MPVT_INTERNAL_TOKEN"); envInternalToken != "" {
		cfg.InternalToken = envInternalToken
	}
	if envUserId := os.Getenv("MPVT_USER_ID"); envUserId != "" {
		cfg.UserId = envUserId
	}
	if envUserEmail := os.Getenv("MPVT_USER_EMAIL"); envUserEmail != "" {
		cfg.UserEmail = envUserEmail
	}
	if envUserRole := os.Getenv("MPVT_USER_ROLE"); envUserRole != "" {
		cfg.UserRole = envUserRole
	}

	return cfg, nil
}

// atomicWriteFile writes data to a temporary file in the target directory and atomically renames it
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// SaveGlobalConfig writes ~/.config/mpvt/config.yaml atomically with 0600 permissions
func SaveGlobalConfig(cfg GlobalConfig) error {
	if _, err := EnsureGlobalConfigDir(); err != nil {
		return err
	}

	path, err := GetGlobalConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get global config path: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal global config: %w", err)
	}

	if err := atomicWriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write global config file %s: %w", path, err)
	}
	return nil
}

// isGitRoot checks if a directory contains a .git entry (file or directory)
func isGitRoot(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// FindLocalConfig recursively checks the current directory and parents for .mpvt/config.json.
// Traversal stops if it reaches a .git root directory or user HOME to prevent ambient inheritance.
func FindLocalConfig() (*LocalConfig, string, error) {
	currDir, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return FindLocalConfigFrom(currDir)
}

// FindLocalConfigFrom searches for .mpvt/config.json upwards starting from startDir
func FindLocalConfigFrom(startDir string) (*LocalConfig, string, error) {
	if realStart, err := filepath.EvalSymlinks(startDir); err == nil {
		startDir = realStart
	}
	currDir := startDir
	var homeDir string
	if h, err := os.UserHomeDir(); err == nil {
		homeDir = filepath.Clean(h)
		if realH, err := filepath.EvalSymlinks(homeDir); err == nil {
			homeDir = realH
		}
	}

	const maxTraversalDepth = 25
	depth := 0

	for depth < maxTraversalDepth {
		depth++
		cleanCurr := filepath.Clean(currDir)
		candidateDir := filepath.Join(cleanCurr, LocalDirName)
		candidateFile := filepath.Join(candidateDir, LocalConfigFile)
		if fi, err := os.Stat(candidateFile); err == nil && !fi.IsDir() {
			data, err := os.ReadFile(candidateFile)
			if err != nil {
				return nil, candidateDir, fmt.Errorf("failed to read %s: %w", candidateFile, err)
			}
			var local LocalConfig
			if err := json.Unmarshal(data, &local); err != nil {
				return nil, candidateDir, fmt.Errorf("failed to parse %s: %w", candidateFile, err)
			}
			return &local, candidateDir, nil
		}

		// Stop upwards directory traversal if current directory is a .git repository root or user HOME
		if isGitRoot(cleanCurr) || (homeDir != "" && isSameDir(cleanCurr, homeDir)) {
			break
		}

		parent := filepath.Dir(cleanCurr)
		if parent == cleanCurr {
			// Reached root
			break
		}
		currDir = parent
	}

	return nil, "", nil
}

// SaveLocalConfig writes .mpvt/config.json in the specified or current directory atomically
func SaveLocalConfig(dir string, local LocalConfig) error {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	mpvtDir := filepath.Join(dir, LocalDirName)
	if err := os.MkdirAll(mpvtDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", mpvtDir, err)
	}
	_ = os.Chmod(mpvtDir, 0700)

	data, err := json.MarshalIndent(local, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal local config: %w", err)
	}

	filePath := filepath.Join(mpvtDir, LocalConfigFile)
	if err := atomicWriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write local config to %s: %w", filePath, err)
	}
	return nil
}

var ErrProjectNotBound = errors.New("no linked project found in current directory. Please run 'mpvt link <slug>' or 'mpvt init'")

// LoadActiveConfig loads both global and local configurations
func LoadActiveConfig() (*Config, error) {
	global, err := LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	local, localPath, err := FindLocalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to find local config: %w", err)
	}

	return &Config{
		Global:    global,
		Local:     local,
		LocalPath: localPath,
	}, nil
}

// GetEffectiveEndpoint returns global/flag endpoint, or DefaultEndpoint if unset.
// Local project configs are not permitted to override the API endpoint for security.
func (c *Config) GetEffectiveEndpoint() string {
	if c.Global.Endpoint != "" {
		return c.Global.Endpoint
	}
	return DefaultEndpoint
}

// EnsureProjectBound returns an error if not in a linked project directory
func (c *Config) EnsureProjectBound() (*LocalConfig, error) {
	if c.Local == nil || c.Local.ProjectId == "" {
		return nil, ErrProjectNotBound
	}
	return c.Local, nil
}
