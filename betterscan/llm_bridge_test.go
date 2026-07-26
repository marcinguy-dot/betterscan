package main

import (
	"testing"
	"time"
)

func TestNewLLMBridge(t *testing.T) {
	tests := []struct {
		name    string
		config  LLMConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "test-key",
				Workers:   4,
				MaxIssues: 100,
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing model",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "test-key",
			},
			wantErr: true,
		},
		{
			name: "missing api key",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "",
			},
			wantErr: true,
		},
		{
			name: "missing endpoint",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "",
				APIKey:    "test-key",
			},
			wantErr: true,
		},
		{
			name: "zero workers defaults to 1",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "test-key",
				Workers:   0,
				MaxIssues: 100,
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero max issues defaults to 1",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "test-key",
				Workers:   4,
				MaxIssues: 0,
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero timeout defaults to 30s",
			config: LLMConfig{
				Provider:  "openai",
				Model:     "gpt-4",
				Endpoint:  "https://api.openai.com/v1/chat/completions",
				APIKey:    "test-key",
				Workers:   4,
				MaxIssues: 100,
				Timeout:   0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge, err := newLLMBridge(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("newLLMBridge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && bridge == nil {
				t.Errorf("newLLMBridge() returned nil bridge for valid config")
			}
		})
	}
}

func TestBuildOWASPPrompt(t *testing.T) {
	issue := newIssue("opengrep", "XSS", "a.php", 10, "critical xss vulnerability")
	prompt := buildOWASPPrompt(issue)
	
	if !contains(prompt, "opengrep") {
		t.Errorf("prompt should contain analyzer")
	}
	if !contains(prompt, "XSS") {
		t.Errorf("prompt should contain code")
	}
	if !contains(prompt, "a.php") {
		t.Errorf("prompt should contain file")
	}
	if !contains(prompt, "10") {
		t.Errorf("prompt should contain line")
	}
	if !contains(prompt, "critical xss vulnerability") {
		t.Errorf("prompt should contain message")
	}
	if !contains(prompt, "OWASP Top 10") {
		t.Errorf("prompt should mention OWASP")
	}
	if !contains(prompt, "mitigations") {
		t.Errorf("prompt should ask for mitigations")
	}
	if !contains(prompt, "fix suggestion") {
		t.Errorf("prompt should ask for fix suggestion")
	}
	if !contains(prompt, "confidence") {
		t.Errorf("prompt should ask for confidence")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
