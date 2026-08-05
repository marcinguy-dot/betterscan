package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"strings"
	"testing"
)

func TestParseToolsEmpty(t *testing.T) {
	selected := parseTools("")
	if len(selected) != 0 {
		t.Fatalf("expected empty set, got %d", len(selected))
	}
}

func TestParseToolsList(t *testing.T) {
	selected := parseTools("opengrep, trivy , bandit")
	if _, ok := selected["opengrep"]; !ok {
		t.Fatalf("missing opengrep")
	}
	if _, ok := selected["trivy"]; !ok {
		t.Fatalf("missing trivy")
	}
	if _, ok := selected["bandit"]; !ok {
		t.Fatalf("missing bandit")
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(selected))
	}
}

func TestAvailableToolsFilters(t *testing.T) {
	selected := map[string]struct{}{
		"opengrep": {},
		"bandit":   {},
	}
	tools := availableTools(selected)
	found := map[string]bool{}
	for _, tool := range tools {
		found[tool.Name] = true
	}
	if !found["opengrep"] || !found["bandit"] {
		t.Fatalf("missing expected tools")
	}
	if found["trivy"] {
		t.Fatalf("unexpected tool trivy in filtered set")
	}
}

func TestDedupeIssues(t *testing.T) {
	issues := []Issue{
		newIssue("opengrep", "XSS", "a.php", 10, "msg"),
		newIssue("opengrep", "XSS", "a.php", 10, "msg"),
		newIssue("bandit", "B001", "a.py", 5, "msg2"),
	}
	deduped, raw := dedupeIssues(issues, DedupeStrict)
	if raw != 3 {
		t.Fatalf("expected raw count 3, got %d", raw)
	}
	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduped issues, got %d", len(deduped))
	}
}

func TestParseCpg(t *testing.T) {
	payload := []byte(`{"findings":[{"code":"CPG_NPE","file":"Main.java","line":12,"message":"Null pointer: example"}],"translation_units":1}`)
	issues := parseCpg(payload, "cpg")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "CPG_NPE" || issues[0].File != "Main.java" || issues[0].Line != 12 {
		t.Fatalf("unexpected issue contents: %+v", issues[0])
	}
}

func TestJavaVersionParsing(t *testing.T) {
	if _, err := javaVersion("/definitely/not/java"); err == nil {
		t.Fatalf("expected error for missing java binary")
	}
}

func TestParseOpenGrep(t *testing.T) {
	payload := []byte(`{"results":[{"check_id":"xss","path":"a.php","start":{"line":12},"extra":{"message":"hi"}}]}`)
	issues := parseOpenGrep(payload, "opengrep")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "xss" || issues[0].File != "a.php" || issues[0].Line != 12 {
		t.Fatalf("unexpected issue contents: %+v", issues[0])
	}
}

func TestParseJoernScan(t *testing.T) {
	stdout := []byte(`
Result: 8.0 : Dangerous function gets() used: /home/user/code/simple.c:6:main
Result: 6.5 : Some other vulnerability: /home/user/code/other.c:12:foo
`)
	issues := parseJoernScan(stdout, nil, "joern")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Code != "Dangerous function gets() used" || issues[0].File != "/home/user/code/simple.c" || issues[0].Line != 6 {
		t.Fatalf("unexpected issue contents: %+v", issues[0])
	}
	if issues[1].Code != "Some other vulnerability" || issues[1].File != "/home/user/code/other.c" || issues[1].Line != 12 {
		t.Fatalf("unexpected issue contents: %+v", issues[1])
	}
}

func TestExtractTarGzPathTraversal(t *testing.T) {
	tmpArchive, err := os.CreateTemp("", "test-traverse-*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpArchive.Name())
	defer tmpArchive.Close()

	gw := gzip.NewWriter(tmpArchive)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name:     "../../outside-file.txt",
		Typeflag: tar.TypeReg,
		Size:     18,
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("malicious content")); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()

	destDir, err := os.MkdirTemp("", "test-dest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(destDir)

	err = extractTarGz(tmpArchive.Name(), destDir)
	if err == nil {
		t.Fatalf("expected error for path traversal entry, got nil")
	}
}

func TestDedupeIssuesFileLine(t *testing.T) {
	issues := []Issue{
		newIssue("opengrep", "XSS", "a.php", 10, "message from opengrep"),
		newIssue("bandit", "B001", "a.py", 5, "message from bandit"),
		newIssue("trivy", "CVE-2021", "a.php", 10, "message from trivy"),
	}
	deduped, raw := dedupeIssues(issues, DedupeFileLine)
	if raw != 3 {
		t.Fatalf("expected raw count 3, got %d", raw)
	}
	// Should collapse to 2 issues (a.php:10 and a.py:5)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduped issues, got %d", len(deduped))
	}
	// Verify the retained message is the longest one
	for _, issue := range deduped {
		if issue.File == "a.php" && issue.Line == 10 {
			if !strings.Contains(issue.Message, "opengrep") {
				t.Errorf("expected longest message retained for a.php:10, got: %s", issue.Message)
			}
		}
	}
}

func TestIssueFingerprint(t *testing.T) {
	issue1 := newIssue("opengrep", "XSS", "a.php", 10, "test message")
	issue2 := newIssue("opengrep", "XSS", "a.php", 10, "test message")
	issue3 := newIssue("opengrep", "XSS", "a.php", 10, "different message")

	if issue1.Fingerprint != issue2.Fingerprint {
		t.Errorf("identical issues should have same fingerprint")
	}
	if issue1.Fingerprint == issue3.Fingerprint {
		t.Errorf("issues with different messages should have different fingerprints")
	}
}
