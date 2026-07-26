package main

import "testing"

func TestValidateRepoURLWorker(t *testing.T) {
	valid := []string{
		"https://github.com/user/repo",
		"https://github.com/user/repo.git",
		"ssh://git@github.com/user/repo.git",
		"git@github.com:user/repo.git",
	}
	for _, v := range valid {
		if _, err := validateRepoURL(v); err != nil {
			t.Errorf("expected %q valid, got %v", v, err)
		}
	}
	invalid := []string{
		"",
		"ext::sh -c id",
		"file:///etc/passwd",
		"-oProxyCommand=evil",
		"https://host/repo with space",
		"https://host/repo\nevil",
	}
	for _, v := range invalid {
		if _, err := validateRepoURL(v); err == nil {
			t.Errorf("expected %q rejected", v)
		}
	}
}

func TestValidateBranchWorker(t *testing.T) {
	if b, err := validateBranch(""); err != nil || b != "main" {
		t.Errorf("empty branch should default to main, got %q %v", b, err)
	}
	for _, v := range []string{"-x", "a..b", "bad branch", "x;rm"} {
		if _, err := validateBranch(v); err == nil {
			t.Errorf("expected branch %q rejected", v)
		}
	}
}

func TestValidateJob(t *testing.T) {
	good := &ScanJob{
		ScanID:     "123e4567-e89b-12d3-a456-426614174000",
		ProjectID:  "123e4567-e89b-12d3-a456-426614174001",
		RepoURL:    "https://github.com/user/repo.git",
		RepoBranch: "",
		Tools:      "trivy, opengrep",
		Strategy:   "",
	}
	if err := validateJob(good); err != nil {
		t.Fatalf("expected job valid: %v", err)
	}
	if good.RepoBranch != "main" {
		t.Errorf("expected branch defaulted to main, got %q", good.RepoBranch)
	}
	if good.Strategy != "parallel" {
		t.Errorf("expected strategy defaulted to parallel, got %q", good.Strategy)
	}
	if good.Tools != "opengrep,trivy" {
		t.Errorf("expected canonical tools, got %q", good.Tools)
	}

	bad := []*ScanJob{
		{ScanID: "nope", ProjectID: "123e4567-e89b-12d3-a456-426614174001", RepoURL: "https://x/y"},
		{ScanID: "123e4567-e89b-12d3-a456-426614174000", ProjectID: "bad", RepoURL: "https://x/y"},
		{ScanID: "123e4567-e89b-12d3-a456-426614174000", ProjectID: "123e4567-e89b-12d3-a456-426614174001", RepoURL: "ext::sh -c id"},
		{ScanID: "123e4567-e89b-12d3-a456-426614174000", ProjectID: "123e4567-e89b-12d3-a456-426614174001", RepoURL: "https://x/y", Tools: "evil"},
	}
	for i, j := range bad {
		if err := validateJob(j); err == nil {
			t.Errorf("expected bad job %d rejected", i)
		}
	}
}
