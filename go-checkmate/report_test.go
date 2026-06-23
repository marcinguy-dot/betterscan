package main

import (
	"os"
	"strings"
	"testing"
)

func TestSeverityToLevel(t *testing.T) {
	tests := []struct {
		message string
		level   string
	}{
		{"critical vulnerability", "error"},
		{"high severity issue", "error"},
		{"medium risk", "warning"},
		{"warning message", "warning"},
		{"low severity", "note"},
		{"info message", "note"},
		{"just a message", "note"},
	}
	for _, tt := range tests {
		if got := severityToLevel(tt.message); got != tt.level {
			t.Errorf("severityToLevel(%q) = %q, want %q", tt.message, got, tt.level)
		}
	}
}

func TestBuildEnrichmentMarkdown(t *testing.T) {
	enrichment := &IssueEnrichment{
		Summary:       "Test summary",
		OWASPCategory: "A03",
		Mitigations:   []string{"mitigation 1", "mitigation 2"},
		FixSuggestion: "fix suggestion",
		Confidence:    "high",
	}
	markdown := buildEnrichmentMarkdown(enrichment)
	
	if !strings.Contains(markdown, "Test summary") {
		t.Errorf("markdown should contain summary")
	}
	if !strings.Contains(markdown, "A03") {
		t.Errorf("markdown should contain OWASP category")
	}
	if !strings.Contains(markdown, "mitigation 1") {
		t.Errorf("markdown should contain mitigation")
	}
	if !strings.Contains(markdown, "fix suggestion") {
		t.Errorf("markdown should contain fix suggestion")
	}
	if !strings.Contains(markdown, "high") {
		t.Errorf("markdown should contain confidence")
	}
}

func TestWriteSARIF(t *testing.T) {
	issues := []Issue{
		newIssue("opengrep", "XSS", "a.php", 10, "critical xss vulnerability"),
		newIssue("bandit", "B001", "a.py", 5, "medium risk issue"),
	}
	
	tempFile, err := os.CreateTemp("", "sarif-test-*.sarif")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	if err := writeSARIF(tempFile.Name(), issues); err != nil {
		t.Fatalf("writeSARIF() error: %v", err)
	}
	
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	
	contentStr := string(content)
	if !strings.Contains(contentStr, "sarif-schema-2.1.0.json") {
		t.Errorf("SARIF should contain schema reference")
	}
	if !strings.Contains(contentStr, "opengrep:XSS") {
		t.Errorf("SARIF should contain rule ID")
	}
	if !strings.Contains(contentStr, "error") {
		t.Errorf("SARIF should contain error level for critical issue")
	}
	if !strings.Contains(contentStr, "warning") {
		t.Errorf("SARIF should contain warning level for medium issue")
	}
}

func TestWriteHTML(t *testing.T) {
	issues := []Issue{
		newIssue("opengrep", "XSS", "a.php", 10, "test message"),
	}
	
	tempFile, err := os.CreateTemp("", "html-test-*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	if err := writeHTML(tempFile.Name(), issues); err != nil {
		t.Fatalf("writeHTML() error: %v", err)
	}
	
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	
	contentStr := string(content)
	if !strings.Contains(contentStr, "<!doctype html>") {
		t.Errorf("HTML should contain doctype")
	}
	if !strings.Contains(contentStr, "go-checkmate Scan Report") {
		t.Errorf("HTML should contain title")
	}
	if !strings.Contains(contentStr, "opengrep") {
		t.Errorf("HTML should contain analyzer name")
	}
	if !strings.Contains(contentStr, "XSS") {
		t.Errorf("HTML should contain code")
	}
}

func TestWriteHTMLWithEnrichment(t *testing.T) {
	issue := newIssue("opengrep", "XSS", "a.php", 10, "test message")
	issue.Enrichment = &IssueEnrichment{
		Summary:       "Enriched summary",
		OWASPCategory: "A03",
		Mitigations:   []string{"mitigation 1"},
		FixSuggestion: "fix suggestion",
		Confidence:    "high",
	}
	
	tempFile, err := os.CreateTemp("", "html-enriched-test-*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	if err := writeHTML(tempFile.Name(), []Issue{issue}); err != nil {
		t.Fatalf("writeHTML() error: %v", err)
	}
	
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	
	contentStr := string(content)
	if !strings.Contains(contentStr, "Enriched summary") {
		t.Errorf("HTML should contain enrichment summary")
	}
	if !strings.Contains(contentStr, "A03") {
		t.Errorf("HTML should contain OWASP category")
	}
	if !strings.Contains(contentStr, "mitigation 1") {
		t.Errorf("HTML should contain mitigation")
	}
}
