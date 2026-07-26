package main

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	maxRepoURLLen = 2048
	maxBranchLen  = 255
)

// allowedTools mirrors the tool names accepted by betterscan's -tools flag.
var allowedTools = map[string]struct{}{
	"opengrep":      {},
	"trivy":         {},
	"bandit":        {},
	"brakeman":      {},
	"gostaticcheck": {},
	"cpg":           {},
	"joern":         {},
}

var allowedStrategies = map[string]struct{}{
	"sequential": {},
	"parallel":   {},
	"both":       {},
}

// allowedRepoSchemes restricts git remotes to expected network transports.
var allowedRepoSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
	"ssh":   {},
	"git":   {},
}

var (
	branchRe  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	scpLikeRe = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+:.+$`)
)

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// validateRepoURL accepts only well-formed git remotes over expected transports.
// This is defense-in-depth: the API validates on ingress, but the worker is the
// component that actually runs `git clone`, so it must independently reject
// option-looking values, control characters and dangerous schemes
// (ext::/file:: etc.) that could lead to command execution.
func validateRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("repo_url is required")
	}
	if len(raw) > maxRepoURLLen {
		return "", errors.New("repo_url is too long")
	}
	if hasControlChars(raw) || strings.ContainsAny(raw, " \t") {
		return "", errors.New("repo_url contains invalid characters")
	}
	if strings.HasPrefix(raw, "-") {
		return "", errors.New("repo_url must not start with '-'")
	}
	if !strings.Contains(raw, "://") {
		if scpLikeRe.MatchString(raw) {
			return raw, nil
		}
		return "", errors.New("repo_url is not a valid git URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("repo_url is not a valid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if _, ok := allowedRepoSchemes[scheme]; !ok {
		return "", fmt.Errorf("repo_url scheme %q is not allowed", scheme)
	}
	if u.Host == "" {
		return "", errors.New("repo_url must include a host")
	}
	return raw, nil
}

// validateBranch accepts a conservative subset of git ref names. Empty input
// returns "main" so the worker always clones a concrete branch.
func validateBranch(branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main", nil
	}
	if len(branch) > maxBranchLen {
		return "", errors.New("repo_branch is too long")
	}
	if strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return "", errors.New("repo_branch has an invalid format")
	}
	if strings.Contains(branch, "..") || strings.HasSuffix(branch, ".lock") {
		return "", errors.New("repo_branch has an invalid format")
	}
	if !branchRe.MatchString(branch) {
		return "", errors.New("repo_branch contains invalid characters")
	}
	return branch, nil
}

// normalizeTools validates a comma-separated tool list against the allowlist and
// returns a canonical, de-duplicated list. Empty input is allowed.
func normalizeTools(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, part := range strings.Split(raw, ",") {
		tool := strings.ToLower(strings.TrimSpace(part))
		if tool == "" {
			continue
		}
		if _, ok := allowedTools[tool]; !ok {
			return "", fmt.Errorf("unknown tool %q", tool)
		}
		if _, dup := seen[tool]; dup {
			continue
		}
		seen[tool] = struct{}{}
		out = append(out, tool)
	}
	sort.Strings(out)
	return strings.Join(out, ","), nil
}

// validateStrategy validates the scan strategy. Empty input defaults to parallel.
func validateStrategy(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "parallel", nil
	}
	if _, ok := allowedStrategies[raw]; !ok {
		return "", fmt.Errorf("invalid strategy %q", raw)
	}
	return raw, nil
}

// validateJob validates and normalizes every field of an incoming scan job. The
// returned job has sanitized values ready to be passed to git/the scanner.
func validateJob(job *ScanJob) error {
	if _, err := uuid.Parse(strings.TrimSpace(job.ScanID)); err != nil {
		return errors.New("invalid scan_id")
	}
	job.ScanID = strings.TrimSpace(job.ScanID)
	if _, err := uuid.Parse(strings.TrimSpace(job.ProjectID)); err != nil {
		return errors.New("invalid project_id")
	}
	job.ProjectID = strings.TrimSpace(job.ProjectID)

	repoURL, err := validateRepoURL(job.RepoURL)
	if err != nil {
		return err
	}
	job.RepoURL = repoURL

	branch, err := validateBranch(job.RepoBranch)
	if err != nil {
		return err
	}
	job.RepoBranch = branch

	tools, err := normalizeTools(job.Tools)
	if err != nil {
		return err
	}
	job.Tools = tools

	strategy, err := validateStrategy(job.Strategy)
	if err != nil {
		return err
	}
	job.Strategy = strategy
	return nil
}
