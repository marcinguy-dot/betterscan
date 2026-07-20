package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestJavaBinaryName(t *testing.T) {
	if runtime.GOOS == "windows" {
		if got := javaBinaryName(); got != "java.exe" {
			t.Errorf("javaBinaryName() = %q, want java.exe", got)
		}
	} else {
		if got := javaBinaryName(); got != "java" {
			t.Errorf("javaBinaryName() = %q, want java", got)
		}
	}
}

func TestJavacBinaryName(t *testing.T) {
	if runtime.GOOS == "windows" {
		if got := javacBinaryName(); got != "javac.exe" {
			t.Errorf("javacBinaryName() = %q, want javac.exe", got)
		}
	} else {
		if got := javacBinaryName(); got != "javac" {
			t.Errorf("javacBinaryName() = %q, want javac", got)
		}
	}
}

func TestCpgHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, ".checkmate", "cpg", cpgVersion)
	got, err := cpgHomeDir()
	if err != nil {
		t.Fatalf("cpgHomeDir() error: %v", err)
	}
	if got != expected {
		t.Errorf("cpgHomeDir() = %q, want %q", got, expected)
	}
}

func TestCpgRunnerPackage(t *testing.T) {
	if cpgRunnerPackage != "com.checkmate.security.Runner" {
		t.Fatalf("cpgRunnerPackage = %q, want com.checkmate.security.Runner", cpgRunnerPackage)
	}
	// Ensure class path layout matches Java package.
	wantSuffix := filepath.Join("com", "checkmate", "security", "Runner.class")
	got := filepath.Join(strings.Split(cpgRunnerPackage, ".")...) + ".class"
	if got != wantSuffix && filepath.FromSlash(strings.ReplaceAll(cpgRunnerPackage, ".", "/")+".class") != wantSuffix {
		// Normalize for OS separators
		normalized := filepath.Join(strings.Split(cpgRunnerPackage, ".")...) + ".class"
		if normalized != wantSuffix {
			t.Fatalf("class path layout %q does not match %q", normalized, wantSuffix)
		}
	}
}

func TestBundledJavaHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, ".checkmate", "java", "jdk-17")
	got, err := bundledJavaHomeDir()
	if err != nil {
		t.Fatalf("bundledJavaHomeDir() error: %v", err)
	}
	if got != expected {
		t.Errorf("bundledJavaHomeDir() = %q, want %q", got, expected)
	}
}

func TestCpgRunnerSourcePath(t *testing.T) {
	// Create a temporary directory structure mimicking the expected layout
	tempDir, err := os.MkdirTemp("", "cpg-runner-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create the expected directory structure
	cpgDir := filepath.Join(tempDir, "cpg", "src", "main", "java", "com", "checkmate", "security")
	if err := os.MkdirAll(cpgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy Runner.java file
	runnerFile := filepath.Join(cpgDir, "Runner.java")
	if err := os.WriteFile(runnerFile, []byte("package com.checkmate.security;"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save current working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Test the path resolution
	got, err := cpgRunnerSourcePath()
	if err != nil {
		t.Fatalf("cpgRunnerSourcePath() error: %v", err)
	}

	// The path should be relative to the current directory
	expected := filepath.Join("cpg", "src", "main", "java", "com", "checkmate", "security", "Runner.java")
	// The function may return an absolute path, so check if it ends with the expected relative path
	if !filepath.IsAbs(got) && got != expected {
		t.Errorf("cpgRunnerSourcePath() = %q, want %q", got, expected)
	}
	if filepath.IsAbs(got) && !strings.HasSuffix(got, expected) {
		t.Errorf("cpgRunnerSourcePath() = %q, should end with %q", got, expected)
	}
}

func TestHasSupportedCpgSources(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected bool
	}{
		{
			name:     "empty directory",
			files:    []string{},
			expected: false,
		},
		{
			name:     "java file",
			files:    []string{"Test.java"},
			expected: true,
		},
		{
			name:     "c file",
			files:    []string{"test.c"},
			expected: true,
		},
		{
			name:     "cpp file",
			files:    []string{"test.cpp"},
			expected: true,
		},
		{
			name:     "go file",
			files:    []string{"test.go"},
			expected: true,
		},
		{
			name:     "python file",
			files:    []string{"test.py"},
			expected: true,
		},
		{
			name:     "unsupported file",
			files:    []string{"test.txt"},
			expected: false,
		},
		{
			name:     "mixed with supported",
			files:    []string{"test.txt", "test.java"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "cpg-sources-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tempDir)

			// Create test files
			for _, file := range tt.files {
				if err := os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := hasSupportedCpgSources(tempDir)
			if err != nil {
				t.Fatalf("hasSupportedCpgSources() error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("hasSupportedCpgSources() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCpgJSONFromMixedOutput(t *testing.T) {
	tests := []struct {
		name     string
		stdout   []byte
		stderr   []byte
		expected []byte
	}{
		{
			name:     "json in stdout",
			stdout:   []byte(`{"findings":[{"code":"TEST"}]}`),
			stderr:   []byte("some log output"),
			expected: []byte(`{"findings":[{"code":"TEST"}]}`),
		},
		{
			name:     "json in stderr",
			stdout:   []byte("some log output"),
			stderr:   []byte(`{"findings":[{"code":"TEST"}]}`),
			expected: []byte(`{"findings":[{"code":"TEST"}]}`),
		},
		{
			name:     "json in combined output",
			stdout:   []byte("log before"),
			stderr:   []byte(`{"findings":[{"code":"TEST"}]}`),
			expected: []byte(`{"findings":[{"code":"TEST"}]}`),
		},
		{
			name:     "no json",
			stdout:   []byte("just logs"),
			stderr:   []byte("more logs"),
			expected: nil,
		},
		{
			name:     "invalid json",
			stdout:   []byte("{invalid"),
			stderr:   []byte(""),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cpgJSONFromMixedOutput(tt.stdout, tt.stderr)
			if (got == nil && tt.expected != nil) || (got != nil && tt.expected == nil) {
				t.Errorf("cpgJSONFromMixedOutput() = %v, want %v", got, tt.expected)
				return
			}
			if got != nil && tt.expected != nil && string(got) != string(tt.expected) {
				t.Errorf("cpgJSONFromMixedOutput() = %q, want %q", string(got), string(tt.expected))
			}
		})
	}
}

func TestCpgInstallPreflight(t *testing.T) {
	// Test that preflight checks platform support
	err := cpgInstallPreflight()
	if err != nil {
		// This should only fail on unsupported platforms
		if !strings.Contains(err.Error(), "unsupported OS") && !strings.Contains(err.Error(), "unsupported architecture") {
			t.Errorf("cpgInstallPreflight() unexpected error: %v", err)
		}
	}
}

func TestAdoptiumPlatform(t *testing.T) {
	osName, arch, err := adoptiumPlatform()
	if err != nil {
		// This should only fail on unsupported platforms
		if !strings.Contains(err.Error(), "unsupported OS") && !strings.Contains(err.Error(), "unsupported architecture") {
			t.Errorf("adoptiumPlatform() unexpected error: %v", err)
		}
		return
	}

	// Validate returned values
	validOS := map[string]bool{"mac": true, "linux": true, "windows": true}
	validArch := map[string]bool{"x64": true, "aarch64": true}

	if !validOS[osName] {
		t.Errorf("adoptiumPlatform() returned invalid OS: %q", osName)
	}
	if !validArch[arch] {
		t.Errorf("adoptiumPlatform() returned invalid arch: %q", arch)
	}
}
