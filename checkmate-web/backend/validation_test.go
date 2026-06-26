package main

import "testing"

func TestValidateRepoURL(t *testing.T) {
	valid := []string{
		"https://github.com/user/repo",
		"https://github.com/user/repo.git",
		"http://example.com/x.git",
		"ssh://git@github.com/user/repo.git",
		"git://github.com/user/repo.git",
		"git@github.com:user/repo.git",
	}
	for _, v := range valid {
		if _, err := validateRepoURL(v); err != nil {
			t.Errorf("expected %q to be valid, got %v", v, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"ext::sh -c 'touch /tmp/pwned'",
		"file:///etc/passwd",
		"fd::17",
		"-oProxyCommand=evil",
		"--upload-pack=evil",
		"https://github.com/user/repo with space",
		"https://github.com/user/repo\nrm -rf",
		"not a url",
		"https://",
	}
	for _, v := range invalid {
		if _, err := validateRepoURL(v); err == nil {
			t.Errorf("expected %q to be rejected", v)
		}
	}
}

func TestValidateBranch(t *testing.T) {
	if b, err := validateBranch(""); err != nil || b != "" {
		t.Errorf("empty branch should be allowed and empty, got %q %v", b, err)
	}
	valid := []string{"main", "release/1.2.3", "feature_x", "v1.0.0-rc.1"}
	for _, v := range valid {
		if _, err := validateBranch(v); err != nil {
			t.Errorf("expected branch %q valid, got %v", v, err)
		}
	}
	invalid := []string{"-x", "/abc", "abc/", "a..b", "x.lock", "bad branch", "evil;rm -rf", "br$(id)"}
	for _, v := range invalid {
		if _, err := validateBranch(v); err == nil {
			t.Errorf("expected branch %q rejected", v)
		}
	}
}

func TestNormalizeTools(t *testing.T) {
	out, err := normalizeTools("trivy, opengrep ,trivy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "opengrep,trivy" {
		t.Errorf("expected canonical 'opengrep,trivy', got %q", out)
	}
	if _, err := normalizeTools(""); err != nil {
		t.Errorf("empty tools should be allowed: %v", err)
	}
	for _, bad := range []string{"rm", "trivy,evil", "../../bin/sh", "trivy;bandit"} {
		if _, err := normalizeTools(bad); err == nil {
			t.Errorf("expected tools %q rejected", bad)
		}
	}
}

func TestValidateStrategy(t *testing.T) {
	if s, err := validateStrategy(""); err != nil || s != "parallel" {
		t.Errorf("empty strategy should default to parallel, got %q %v", s, err)
	}
	for _, v := range []string{"sequential", "parallel", "both", "BOTH"} {
		if _, err := validateStrategy(v); err != nil {
			t.Errorf("expected strategy %q valid: %v", v, err)
		}
	}
	for _, v := range []string{"fast", "evil;", "rm"} {
		if _, err := validateStrategy(v); err == nil {
			t.Errorf("expected strategy %q rejected", v)
		}
	}
}

func TestValidateSeverity(t *testing.T) {
	if s, err := validateSeverity(""); err != nil || s != "" {
		t.Errorf("empty severity should be allowed: %q %v", s, err)
	}
	for _, v := range []string{"critical", "high", "medium", "low"} {
		if _, err := validateSeverity(v); err != nil {
			t.Errorf("expected severity %q valid: %v", v, err)
		}
	}
	if _, err := validateSeverity("' OR 1=1 --"); err == nil {
		t.Errorf("expected sqli-looking severity rejected")
	}
}

func TestValidateCronExpr(t *testing.T) {
	valid := []string{
		"* * * * *",
		"0 0 * * *",
		"*/15 * * * *",
		"0 9-17 * * 1-5",
		"0,30 8 1 1 0",
		"@daily",
		"@hourly",
	}
	for _, v := range valid {
		if _, err := validateCronExpr(v); err != nil {
			t.Errorf("expected cron %q valid: %v", v, err)
		}
	}
	invalid := []string{
		"",
		"* * * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * * 13 *",
		"* * * * 7",
		"@weeklyish",
		"a b c d e",
		"*/0 * * * *",
	}
	for _, v := range invalid {
		if _, err := validateCronExpr(v); err == nil {
			t.Errorf("expected cron %q rejected", v)
		}
	}
}

func TestValidateUUID(t *testing.T) {
	if _, err := validateUUID("123e4567-e89b-12d3-a456-426614174000"); err != nil {
		t.Errorf("expected valid uuid: %v", err)
	}
	for _, v := range []string{"", "not-a-uuid", "1; DROP TABLE", "../etc"} {
		if _, err := validateUUID(v); err == nil {
			t.Errorf("expected uuid %q rejected", v)
		}
	}
}

func TestTextLimits(t *testing.T) {
	if _, err := validateName(""); err == nil {
		t.Error("empty name should be rejected")
	}
	long := make([]byte, maxNameLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := validateName(string(long)); err == nil {
		t.Error("over-long name should be rejected")
	}
	if _, err := validateName("ok\x00name"); err == nil {
		t.Error("name with control char should be rejected")
	}
}
