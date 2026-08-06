package main

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Limits for free-text fields. These bound storage/rendering and stop a client
// from sending arbitrarily large payloads.
const (
	maxNameLen        = 200
	maxDescriptionLen = 5000
	maxLanguageLen    = 50
	maxRepoURLLen     = 2048
	maxBranchLen      = 255
	maxReasonLen      = 2000
	maxCronExprLen    = 256
	maxEmailLen       = 254
	minPasswordLen    = 8
	maxPasswordLen    = 128
)

// allowedTools mirrors the tool names accepted by betterscan's -tools flag.
// Anything outside this set is rejected before it can reach the scanner binary.
var allowedTools = map[string]struct{}{
	"opengrep":      {},
	"trivy":         {},
	"bandit":        {},
	"gostaticcheck": {},
	"cpg":           {},
	"joern":         {},
}

// allowedStrategies mirrors betterscan's -strategy flag.
var allowedStrategies = map[string]struct{}{
	"sequential": {},
	"parallel":   {},
	"both":       {},
}

// allowedSeverities are the only severities findings can be filtered by.
var allowedSeverities = map[string]struct{}{
	"critical": {},
	"high":     {},
	"medium":   {},
	"low":      {},
}

// allowedRepoSchemes restricts git remotes to network transports we expect.
// ext::/file::/fd:: and similar are intentionally excluded because they let an
// attacker turn a clone into arbitrary command execution or local file access.
var allowedRepoSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
	"ssh":   {},
	"git":   {},
}

var (
	branchRe   = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	languageRe = regexp.MustCompile(`^[A-Za-z0-9 +#._-]+$`)
	scpLikeRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+:.+$`)
	emailRe    = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)
)

// validateEmail normalizes and validates a login email address.
func validateEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > maxEmailLen {
		return "", fmt.Errorf("email must be at most %d characters", maxEmailLen)
	}
	if !emailRe.MatchString(email) {
		return "", errors.New("email is not valid")
	}
	return email, nil
}

// validatePassword enforces a minimum strength without rejecting any byte the
// user might legitimately use in a passphrase. It is not trimmed: leading and
// trailing whitespace are significant in passwords.
func validatePassword(raw string) (string, error) {
	if len(raw) < minPasswordLen {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	if len(raw) > maxPasswordLen {
		return "", fmt.Errorf("password must be at most %d characters", maxPasswordLen)
	}
	return raw, nil
}

// hasControlChars reports whether s contains C0/C1 control characters. Tab and
// newline are allowed for multi-line fields when allowNewlines is true.
func hasControlChars(s string, allowNewlines bool) bool {
	for _, r := range s {
		if r == '\t' || (allowNewlines && (r == '\n' || r == '\r')) {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// validateName ensures a required, single-line, bounded display name.
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > maxNameLen {
		return "", fmt.Errorf("name must be at most %d characters", maxNameLen)
	}
	if hasControlChars(name, false) {
		return "", errors.New("name contains invalid control characters")
	}
	return name, nil
}

// validateDescription bounds an optional, possibly multi-line description.
func validateDescription(desc string) (string, error) {
	if len(desc) > maxDescriptionLen {
		return "", fmt.Errorf("description must be at most %d characters", maxDescriptionLen)
	}
	if hasControlChars(desc, true) {
		return "", errors.New("description contains invalid control characters")
	}
	return desc, nil
}

// validateLanguage bounds the optional language tag to a safe charset.
func validateLanguage(lang string) (string, error) {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "", nil
	}
	if len(lang) > maxLanguageLen {
		return "", fmt.Errorf("language must be at most %d characters", maxLanguageLen)
	}
	if !languageRe.MatchString(lang) {
		return "", errors.New("language contains invalid characters")
	}
	return lang, nil
}

// validateReason bounds the optional false-positive justification text.
func validateReason(reason string) (string, error) {
	if len(reason) > maxReasonLen {
		return "", fmt.Errorf("reason must be at most %d characters", maxReasonLen)
	}
	if hasControlChars(reason, true) {
		return "", errors.New("reason contains invalid control characters")
	}
	return reason, nil
}

// validateRepoURL accepts only well-formed git remotes over expected transports.
// It rejects option-looking values, control characters and dangerous schemes so
// the URL cannot be abused for argument or command injection during clone.
func validateRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("repo_url is required")
	}
	if len(raw) > maxRepoURLLen {
		return "", fmt.Errorf("repo_url must be at most %d characters", maxRepoURLLen)
	}
	if hasControlChars(raw, false) || strings.ContainsAny(raw, " \t") {
		return "", errors.New("repo_url contains invalid characters")
	}
	if strings.HasPrefix(raw, "-") {
		return "", errors.New("repo_url must not start with '-'")
	}

	// scp-like syntax (git@host:path) has no scheme.
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
// returns the empty string so callers can fall back to a default branch.
func validateBranch(branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", nil
	}
	if len(branch) > maxBranchLen {
		return "", fmt.Errorf("repo_branch must be at most %d characters", maxBranchLen)
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
// returns a canonical, de-duplicated, lower-cased list. Empty input is allowed
// and means "let the scanner use its defaults".
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

// validateSeverity validates an optional severity filter value.
func validateSeverity(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedSeverities[raw]; !ok {
		return "", fmt.Errorf("invalid severity %q", raw)
	}
	return raw, nil
}

// validateUUID parses a string as a UUID, rejecting anything malformed.
func validateUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, errors.New("invalid id format")
	}
	return id, nil
}

// validateBool ensures an optional boolean query value is exactly true/false.
func validateBool(raw string) (bool, bool, error) {
	if raw == "" {
		return false, false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false, errors.New("value must be true or false")
	}
	return v, true, nil
}

var cronMacros = map[string]struct{}{
	"@yearly":   {},
	"@annually": {},
	"@monthly":  {},
	"@weekly":   {},
	"@daily":    {},
	"@midnight": {},
	"@hourly":   {},
}

type cronRange struct{ min, max int }

var cronFieldRanges = []cronRange{
	{0, 59}, // minute
	{0, 23}, // hour
	{1, 31}, // day of month
	{1, 12}, // month
	{0, 6},  // day of week
}

// validateCronExpr validates a standard 5-field cron expression or a supported
// @macro. It rejects malformed expressions and out-of-range values so a bad
// schedule cannot be persisted.
func validateCronExpr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("cron_expr is required")
	}
	if len(raw) > maxCronExprLen {
		return "", fmt.Errorf("cron_expr must be at most %d characters", maxCronExprLen)
	}
	if strings.HasPrefix(raw, "@") {
		if _, ok := cronMacros[strings.ToLower(raw)]; !ok {
			return "", fmt.Errorf("unsupported cron macro %q", raw)
		}
		return strings.ToLower(raw), nil
	}

	fields := strings.Fields(raw)
	if len(fields) != 5 {
		return "", errors.New("cron_expr must have 5 fields")
	}
	for i, field := range fields {
		if err := validateCronField(field, cronFieldRanges[i]); err != nil {
			return "", fmt.Errorf("cron_expr field %d: %w", i+1, err)
		}
	}
	return strings.Join(fields, " "), nil
}

func validateCronField(field string, r cronRange) error {
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return errors.New("empty value")
		}
		// Optional step: value/step
		stepValue := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			stepValue = part[:idx]
			step := part[idx+1:]
			n, err := strconv.Atoi(step)
			if err != nil || n <= 0 {
				return errors.New("invalid step")
			}
		}
		if stepValue == "*" {
			continue
		}
		// Range a-b or single value.
		if idx := strings.Index(stepValue, "-"); idx >= 0 {
			lo, errLo := strconv.Atoi(stepValue[:idx])
			hi, errHi := strconv.Atoi(stepValue[idx+1:])
			if errLo != nil || errHi != nil {
				return errors.New("invalid range")
			}
			if lo < r.min || hi > r.max || lo > hi {
				return errors.New("value out of range")
			}
			continue
		}
		n, err := strconv.Atoi(stepValue)
		if err != nil {
			return errors.New("invalid value")
		}
		if n < r.min || n > r.max {
			return errors.New("value out of range")
		}
	}
	return nil
}
