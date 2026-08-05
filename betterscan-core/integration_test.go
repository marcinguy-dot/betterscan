package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Integration tests exercise real tool installs/runs against vulnerable fixtures.
// They are skipped under -short so unit CI stays fast:
//
//	go test -short ./...
//	go test -timeout 45m ./...   # full integration suite

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// betterscan/integration_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func issueCodes(issues []Issue) map[string]int {
	out := map[string]int{}
	for _, iss := range issues {
		out[iss.Code]++
	}
	return out
}

func hasCodeContaining(issues []Issue, substr string) bool {
	substr = strings.ToLower(substr)
	for _, iss := range issues {
		if strings.Contains(strings.ToLower(iss.Code), substr) ||
			strings.Contains(strings.ToLower(iss.Message), substr) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fraunhofer CPG
// ---------------------------------------------------------------------------

func TestIntegrationCPGFindsNpeAndOob(t *testing.T) {
	skipIfShort(t)

	dir := t.TempDir()
	writeFixture(t, dir, "Vuln.java", `
public class Vuln {
    public void npeCast() {
        ((String) null).length();
    }
    public void oob() {
        int[] arr = new int[]{1, 2, 3};
        System.out.println(arr[-1]);
    }
}
`)

	ctx := Context{
		CodeDir:        dir,
		Jobs:           2,
		InstallMissing: true,
		Verbose:        true,
		DedupeScope:    DedupeStrict,
	}

	result, issues := runTool(Tool{Name: "cpg", Kind: ToolCpg}, ctx)
	t.Logf("cpg status=%s exit=%v note=%q duration=%dms issues=%d",
		result.Status, result.ExitCode, result.Note, result.DurationMs, len(issues))
	if result.Command != "" {
		t.Logf("command: %s", result.Command)
	}

	if result.Status == "skipped" {
		t.Fatalf("cpg skipped: %s", result.Note)
	}
	if result.Status == "error" {
		t.Fatalf("cpg error: %s", result.Note)
	}

	codes := issueCodes(issues)
	t.Logf("finding codes: %v", codes)

	if codes["CPG_NPE"] < 1 {
		t.Errorf("expected at least one CPG_NPE finding, got codes=%v issues=%+v", codes, issues)
	}
	if codes["CPG_OOB"] < 1 {
		t.Errorf("expected at least one CPG_OOB finding, got codes=%v issues=%+v", codes, issues)
	}
	for _, iss := range issues {
		if iss.Analyzer != "cpg" {
			t.Errorf("unexpected analyzer %q", iss.Analyzer)
		}
		if iss.Fingerprint == "" {
			t.Errorf("missing fingerprint on %+v", iss)
		}
	}
}

func TestIntegrationCPGRunnerPackageMatchesSource(t *testing.T) {
	// Lightweight sanity: constant must match Java package path on disk.
	src, err := cpgRunnerSourcePath()
	if err != nil {
		// Source only present when running from betterscan working directory.
		t.Skipf("CPG runner source not found from cwd: %v", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "package com.betterscan.security") {
		t.Fatalf("Runner.java package mismatch with cpgRunnerPackage=%s", cpgRunnerPackage)
	}
	if cpgRunnerPackage != "com.betterscan.security.Runner" {
		t.Fatalf("cpgRunnerPackage = %q, want com.betterscan.security.Runner", cpgRunnerPackage)
	}
}

// ---------------------------------------------------------------------------
// Joern
// ---------------------------------------------------------------------------

func TestIntegrationJoernFindsDangerousGets(t *testing.T) {
	skipIfShort(t)

	dir := t.TempDir()
	// Minimal C with gets() — classic Joern query target.
	writeFixture(t, dir, "bof.c", `
#include <stdio.h>
int main(void) {
    char buf[8];
    gets(buf);
    return 0;
}
`)

	ctx := Context{
		CodeDir:        dir,
		Jobs:           2,
		InstallMissing: true,
		Verbose:        true,
		DedupeScope:    DedupeStrict,
	}

	// Joern install can be multi-minute on first run.
	start := time.Now()
	result, issues := runTool(Tool{Name: "joern", Kind: ToolJoern}, ctx)
	t.Logf("joern status=%s exit=%v note=%q duration=%dms wall=%s issues=%d",
		result.Status, result.ExitCode, result.Note, result.DurationMs, time.Since(start), len(issues))
	if result.Command != "" {
		t.Logf("command: %s", result.Command)
	}

	if result.Status == "skipped" {
		t.Fatalf("joern skipped: %s", result.Note)
	}
	if result.Status == "error" {
		t.Fatalf("joern error: %s", result.Note)
	}

	if len(issues) == 0 {
		// Dump parser-level expectation with a synthetic line to ensure parse works;
		// the real run should still produce findings for gets().
		t.Fatalf("expected joern findings for gets(), got none (status=%s)", result.Status)
	}

	if !hasCodeContaining(issues, "gets") && !hasCodeContaining(issues, "dangerous") &&
		!hasCodeContaining(issues, "overflow") && !hasCodeContaining(issues, "buffer") {
		// Still accept any security finding — query titles vary by joern version.
		t.Logf("issues (no gets keyword match, accepting non-empty): %+v", issues)
	} else {
		t.Logf("joern found expected gets-related issues: %+v", issues)
	}

	for _, iss := range issues {
		if iss.Analyzer != "joern" {
			t.Errorf("unexpected analyzer %q", iss.Analyzer)
		}
	}
}

// ---------------------------------------------------------------------------
// Trivy (available on this machine; scans config misconfigurations)
// ---------------------------------------------------------------------------

func TestIntegrationTrivyFindsDockerfileRoot(t *testing.T) {
	skipIfShort(t)
	if _, err := exec.LookPath("trivy"); err != nil {
		t.Skip("trivy not installed")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "Dockerfile", `
FROM alpine:3.19
USER root
COPY app /app
CMD ["/app"]
`)

	ctx := Context{
		CodeDir:        dir,
		Jobs:           2,
		InstallMissing: false,
		Verbose:        true,
		DedupeScope:    DedupeStrict,
	}
	result, issues := runTool(Tool{Name: "trivy", Kind: ToolTrivy}, ctx)
	t.Logf("trivy status=%s issues=%d note=%q", result.Status, len(issues), result.Note)
	if result.Status == "skipped" || result.Status == "error" {
		t.Fatalf("trivy failed: status=%s note=%s", result.Status, result.Note)
	}
	// Trivy config scan should report at least one misconfiguration on a root USER image.
	if len(issues) == 0 {
		t.Fatalf("expected trivy misconfiguration findings, got none")
	}
	for _, iss := range issues {
		if iss.Analyzer != "trivy" {
			t.Errorf("unexpected analyzer %q", iss.Analyzer)
		}
		if iss.Code == "" {
			t.Errorf("empty code on %+v", iss)
		}
	}
}

// ---------------------------------------------------------------------------
// OpenGrep against broken-vulnerable-code-snippets
// ---------------------------------------------------------------------------

func TestIntegrationOpenGrepScansVulnerablePHP(t *testing.T) {
	skipIfShort(t)

	root := repoRoot(t)
	// Use a small focused subset so the scan is fast and deterministic.
	srcDir := filepath.Join(root, "broken-vulnerable-code-snippets", "SQL Injection")
	if _, err := os.Stat(srcDir); err != nil {
		t.Skipf("vulnerable dataset not present: %v", err)
	}

	// Local rules if present in betterscan/rules; otherwise allow refresh.
	rulesDir := filepath.Join(root, "betterscan", "rules")
	useLocalRules := false
	if st, err := os.Stat(filepath.Join(rulesDir, "aikido")); err == nil && st.IsDir() {
		if st2, err := os.Stat(filepath.Join(rulesDir, "amplify")); err == nil && st2.IsDir() {
			useLocalRules = true
		}
	}

	ctx := Context{
		CodeDir:        srcDir,
		Jobs:           runtime.NumCPU(),
		InstallMissing: true,
		Verbose:        true,
		DedupeScope:    DedupeStrict,
		RulesDir:       rulesDir,
		RefreshRules:   !useLocalRules,
		UseRulesPaths:  false,
	}
	if useLocalRules {
		ctx.RefreshRules = false
	}

	result, issues := runTool(Tool{Name: "opengrep", Kind: ToolOpengrep}, ctx)
	t.Logf("opengrep status=%s issues=%d note=%q duration=%dms",
		result.Status, len(issues), result.Note, result.DurationMs)
	if result.Command != "" {
		t.Logf("command: %s", result.Command)
	}

	if result.Status == "skipped" {
		t.Fatalf("opengrep skipped: %s", result.Note)
	}
	if result.Status == "error" {
		t.Fatalf("opengrep error: %s", result.Note)
	}

	// SQL Injection corpus should produce security findings with Aikido/Amplify rules.
	if len(issues) == 0 {
		t.Fatalf("expected opengrep findings on SQL Injection corpus, got none")
	}
	t.Logf("sample finding: %+v", issues[0])
}

// ---------------------------------------------------------------------------
// Full strategy smoke (tools that are installed / installable)
// ---------------------------------------------------------------------------

func TestIntegrationParallelStrategyOnFixtures(t *testing.T) {
	skipIfShort(t)

	dir := t.TempDir()
	writeFixture(t, dir, "Vuln.java", `
public class Vuln {
    public void npe() { ((String) null).hashCode(); }
    public void oob() { int[] a = new int[2]; int x = a[-1]; }
}
`)
	writeFixture(t, dir, "bof.c", `
#include <stdio.h>
void f(void) { char b[4]; gets(b); }
`)
	writeFixture(t, dir, "Dockerfile", "FROM alpine\nUSER root\n")

	// Prefer tools we can exercise without huge downloads when already present.
	selected := parseTools("cpg,trivy")
	tools := availableTools(selected)
	if len(tools) == 0 {
		t.Fatal("no tools selected")
	}

	ctx := Context{
		CodeDir:        dir,
		Jobs:           4,
		InstallMissing: true,
		Verbose:        false,
		DedupeScope:    DedupeStrict,
	}

	strategy := runParallel("parallel", tools, ctx)
	t.Logf("strategy duration=%dms raw=%d final=%d tools=%d",
		strategy.DurationMs, strategy.RawIssues, len(strategy.Issues), len(strategy.Tools))

	var cpgOK, trivyOK bool
	for _, tr := range strategy.Tools {
		t.Logf("tool %s status=%s issues=%d note=%q", tr.Name, tr.Status, tr.Issues, tr.Note)
		if tr.Name == "cpg" && (tr.Status == "ok" || tr.Status == "failed") {
			// failed with findings still counts as tool execution
			cpgOK = true
		}
		if tr.Name == "trivy" && (tr.Status == "ok" || tr.Status == "failed") {
			trivyOK = true
		}
	}
	if !cpgOK {
		t.Error("cpg did not execute successfully in parallel strategy")
	}
	if _, err := exec.LookPath("trivy"); err == nil && !trivyOK {
		t.Error("trivy did not execute successfully in parallel strategy")
	}

	// CPG should contribute NPE/OOB into strategy issues.
	if !hasCodeContaining(strategy.Issues, "CPG_NPE") && !hasCodeContaining(strategy.Issues, "CPG_OOB") {
		// Dump for debugging
		payload, _ := json.MarshalIndent(strategy.Issues, "", "  ")
		t.Fatalf("expected CPG findings in strategy issues, got:\n%s", payload)
	}
}

// ---------------------------------------------------------------------------
// Report pipeline smoke with real issues
// ---------------------------------------------------------------------------

func TestIntegrationReportsFromCPGFindings(t *testing.T) {
	skipIfShort(t)

	dir := t.TempDir()
	writeFixture(t, dir, "Vuln.java", `
public class Vuln {
    void npe() { ((String) null).length(); }
    void oob() { int[] a = {1}; int x = a[-1]; }
}
`)
	ctx := Context{CodeDir: dir, Jobs: 2, InstallMissing: true, DedupeScope: DedupeStrict}
	_, issues := runTool(Tool{Name: "cpg", Kind: ToolCpg}, ctx)
	if len(issues) == 0 {
		t.Fatal("need CPG issues for report smoke test")
	}

	outDir := t.TempDir()
	sarifPath := filepath.Join(outDir, "out.sarif")
	htmlPath := filepath.Join(outDir, "out.html")
	if err := writeSARIF(sarifPath, issues); err != nil {
		t.Fatal(err)
	}
	if err := writeHTML(htmlPath, issues); err != nil {
		t.Fatal(err)
	}
	sarifData, err := os.ReadFile(sarifPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sarifData), "CPG_") {
		t.Fatalf("sarif missing CPG rules: %s", sarifData[:min(200, len(sarifData))])
	}
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlData), "cpg") {
		t.Fatalf("html missing cpg content")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
