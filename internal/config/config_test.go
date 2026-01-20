package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "tilde alone",
			input: "~",
			want:  homeDir,
		},
		{
			name:  "tilde with path",
			input: "~/Documents",
			want:  filepath.Join(homeDir, "Documents"),
		},
		{
			name:  "tilde with nested path",
			input: "~/foo/bar/baz",
			want:  filepath.Join(homeDir, "foo/bar/baz"),
		},
		{
			name:  "absolute path unchanged",
			input: "/usr/local/bin",
			want:  "/usr/local/bin",
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			want:  "relative/path",
		},
		{
			name:  "tilde in middle not expanded",
			input: "/some/path/~user/file",
			want:  "/some/path/~user/file",
		},
		{
			name:  "tilde at end not expanded",
			input: "/some/path~",
			want:  "/some/path~",
		},
		{
			name:  "dot path unchanged",
			input: "./relative",
			want:  "./relative",
		},
		{
			name:  "double dot path unchanged",
			input: "../parent/dir",
			want:  "../parent/dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.input)
			if got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPathWithSlash(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	// Test that ~/path expands correctly (slash after tilde)
	input := "~/.config/activity"
	got := expandPath(input)

	// The result should start with the home directory
	if !strings.HasPrefix(got, homeDir) {
		t.Errorf("expandPath(%q) = %q, expected to start with %q", input, got, homeDir)
	}

	// The result should end with the config path
	if !strings.HasSuffix(got, ".config/activity") && !strings.HasSuffix(got, ".config"+string(filepath.Separator)+"activity") {
		t.Errorf("expandPath(%q) = %q, expected to end with config path", input, got)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Check LLM defaults
	if cfg.LLM.Provider != "gemini" {
		t.Errorf("default LLM.Provider = %q, want %q", cfg.LLM.Provider, "gemini")
	}
	if cfg.LLM.MaxCommits != 50 {
		t.Errorf("default LLM.MaxCommits = %d, want 50", cfg.LLM.MaxCommits)
	}
	if cfg.LLM.MaxMessageLength != 1000 {
		t.Errorf("default LLM.MaxMessageLength = %d, want 1000", cfg.LLM.MaxMessageLength)
	}
	if !cfg.LLM.UseAgent {
		t.Error("default LLM.UseAgent should be true")
	}
	if cfg.LLM.MaxDiffFetches != 5 {
		t.Errorf("default LLM.MaxDiffFetches = %d, want 5", cfg.LLM.MaxDiffFetches)
	}
	if cfg.LLM.MaxDiffSizeKB != 10 {
		t.Errorf("default LLM.MaxDiffSizeKB = %d, want 10", cfg.LLM.MaxDiffSizeKB)
	}
	if cfg.LLM.MaxTotalTokens != 100000 {
		t.Errorf("default LLM.MaxTotalTokens = %d, want 100000", cfg.LLM.MaxTotalTokens)
	}

	// Check Newsletter defaults
	if cfg.Newsletter.Enabled {
		t.Error("default Newsletter.Enabled should be false")
	}
	if cfg.Newsletter.SendGridKeyEnv != "SENDGRID_API_KEY" {
		t.Errorf("default Newsletter.SendGridKeyEnv = %q, want %q",
			cfg.Newsletter.SendGridKeyEnv, "SENDGRID_API_KEY")
	}
}

func TestGetPhase2Prompt(t *testing.T) {
	// Test default prompt
	cfg := DefaultConfig()
	defaultPrompt := cfg.GetPhase2Prompt()
	if defaultPrompt == "" {
		t.Error("GetPhase2Prompt() with no custom prompt returned empty string")
	}
	if defaultPrompt != DefaultPhase2Prompt {
		t.Error("GetPhase2Prompt() with no custom prompt should return DefaultPhase2Prompt")
	}

	// Test custom prompt
	customPrompt := "My custom prompt"
	cfg.LLM.Phase2Prompt = customPrompt
	if got := cfg.GetPhase2Prompt(); got != customPrompt {
		t.Errorf("GetPhase2Prompt() with custom prompt = %q, want %q", got, customPrompt)
	}
}

func TestGetAgentSystemPrompt(t *testing.T) {
	// Test default prompt
	cfg := DefaultConfig()
	defaultPrompt := cfg.GetAgentSystemPrompt()
	if defaultPrompt == "" {
		t.Error("GetAgentSystemPrompt() with no custom prompt returned empty string")
	}
	if defaultPrompt != DefaultAgentSystemPrompt {
		t.Error("GetAgentSystemPrompt() with no custom prompt should return DefaultAgentSystemPrompt")
	}

	// Test custom prompt
	customPrompt := "My custom agent prompt"
	cfg.LLM.AgentSystemPrompt = customPrompt
	if got := cfg.GetAgentSystemPrompt(); got != customPrompt {
		t.Errorf("GetAgentSystemPrompt() with custom prompt = %q, want %q", got, customPrompt)
	}
}

func TestHasGitHubApp(t *testing.T) {
	tests := []struct {
		name           string
		appID          int64
		installationID int64
		want           bool
	}{
		{"both set", 12345, 67890, true},
		{"app id zero", 0, 67890, false},
		{"installation id zero", 12345, 0, false},
		{"both zero", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GitHub: GitHubConfig{
					AppID:          tt.appID,
					InstallationID: tt.installationID,
				},
			}
			if got := cfg.HasGitHubApp(); got != tt.want {
				t.Errorf("HasGitHubApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSendGridAPIKey(t *testing.T) {
	// Test direct key takes precedence
	cfg := &Config{
		Newsletter: NewsletterConfig{
			SendGridAPIKey: "direct-key",
			SendGridKeyEnv: "TEST_SENDGRID_KEY",
		},
	}
	if got := cfg.GetSendGridAPIKey(); got != "direct-key" {
		t.Errorf("GetSendGridAPIKey() with direct key = %q, want %q", got, "direct-key")
	}

	// Test env var fallback
	cfg = &Config{
		Newsletter: NewsletterConfig{
			SendGridKeyEnv: "TEST_SENDGRID_KEY_FOR_TEST",
		},
	}
	os.Setenv("TEST_SENDGRID_KEY_FOR_TEST", "env-key")
	defer os.Unsetenv("TEST_SENDGRID_KEY_FOR_TEST")

	if got := cfg.GetSendGridAPIKey(); got != "env-key" {
		t.Errorf("GetSendGridAPIKey() with env var = %q, want %q", got, "env-key")
	}

	// Test empty when nothing configured
	cfg = &Config{
		Newsletter: NewsletterConfig{},
	}
	if got := cfg.GetSendGridAPIKey(); got != "" {
		t.Errorf("GetSendGridAPIKey() with nothing configured = %q, want empty string", got)
	}
}

func TestGetGitHubAppID(t *testing.T) {
	// Test direct value takes precedence
	cfg := &Config{
		GitHub: GitHubConfig{
			AppID:    12345,
			AppIDEnv: "TEST_GITHUB_APP_ID",
		},
	}
	os.Setenv("TEST_GITHUB_APP_ID", "99999")
	defer os.Unsetenv("TEST_GITHUB_APP_ID")

	if got := cfg.GetGitHubAppID(); got != 12345 {
		t.Errorf("GetGitHubAppID() with direct value = %d, want 12345", got)
	}

	// Test env var fallback
	cfg = &Config{
		GitHub: GitHubConfig{
			AppIDEnv: "TEST_GITHUB_APP_ID_FALLBACK",
		},
	}
	os.Setenv("TEST_GITHUB_APP_ID_FALLBACK", "67890")
	defer os.Unsetenv("TEST_GITHUB_APP_ID_FALLBACK")

	if got := cfg.GetGitHubAppID(); got != 67890 {
		t.Errorf("GetGitHubAppID() with env var = %d, want 67890", got)
	}

	// Test zero when nothing configured
	cfg = &Config{
		GitHub: GitHubConfig{},
	}
	if got := cfg.GetGitHubAppID(); got != 0 {
		t.Errorf("GetGitHubAppID() with nothing configured = %d, want 0", got)
	}

	// Test invalid env var value
	cfg = &Config{
		GitHub: GitHubConfig{
			AppIDEnv: "TEST_GITHUB_APP_ID_INVALID",
		},
	}
	os.Setenv("TEST_GITHUB_APP_ID_INVALID", "not-a-number")
	defer os.Unsetenv("TEST_GITHUB_APP_ID_INVALID")

	if got := cfg.GetGitHubAppID(); got != 0 {
		t.Errorf("GetGitHubAppID() with invalid env var = %d, want 0", got)
	}
}

func TestGetGitHubInstallationID(t *testing.T) {
	// Test direct value takes precedence
	cfg := &Config{
		GitHub: GitHubConfig{
			InstallationID:    54321,
			InstallationIDEnv: "TEST_GITHUB_INSTALLATION_ID",
		},
	}
	os.Setenv("TEST_GITHUB_INSTALLATION_ID", "88888")
	defer os.Unsetenv("TEST_GITHUB_INSTALLATION_ID")

	if got := cfg.GetGitHubInstallationID(); got != 54321 {
		t.Errorf("GetGitHubInstallationID() with direct value = %d, want 54321", got)
	}

	// Test env var fallback
	cfg = &Config{
		GitHub: GitHubConfig{
			InstallationIDEnv: "TEST_GITHUB_INSTALLATION_ID_FALLBACK",
		},
	}
	os.Setenv("TEST_GITHUB_INSTALLATION_ID_FALLBACK", "11111")
	defer os.Unsetenv("TEST_GITHUB_INSTALLATION_ID_FALLBACK")

	if got := cfg.GetGitHubInstallationID(); got != 11111 {
		t.Errorf("GetGitHubInstallationID() with env var = %d, want 11111", got)
	}

	// Test zero when nothing configured
	cfg = &Config{
		GitHub: GitHubConfig{},
	}
	if got := cfg.GetGitHubInstallationID(); got != 0 {
		t.Errorf("GetGitHubInstallationID() with nothing configured = %d, want 0", got)
	}
}

func TestHasGitHubAppWithEnvVars(t *testing.T) {
	// Test HasGitHubApp with env vars
	cfg := &Config{
		GitHub: GitHubConfig{
			AppIDEnv:          "TEST_HAS_GITHUB_APP_ID",
			InstallationIDEnv: "TEST_HAS_GITHUB_INSTALLATION_ID",
		},
	}

	// Not configured yet
	if cfg.HasGitHubApp() {
		t.Error("HasGitHubApp() should be false when env vars not set")
	}

	// Set env vars
	os.Setenv("TEST_HAS_GITHUB_APP_ID", "12345")
	os.Setenv("TEST_HAS_GITHUB_INSTALLATION_ID", "67890")
	defer os.Unsetenv("TEST_HAS_GITHUB_APP_ID")
	defer os.Unsetenv("TEST_HAS_GITHUB_INSTALLATION_ID")

	if !cfg.HasGitHubApp() {
		t.Error("HasGitHubApp() should be true when env vars are set")
	}
}
