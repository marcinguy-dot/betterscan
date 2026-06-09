package main

import "testing"

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
