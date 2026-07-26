package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	aikidoRulesURL  = "https://github.com/AikidoSec/opengrep-rules/archive/refs/heads/main.tar.gz"
	amplifyRulesURL = "https://github.com/amplify-security/opengrep-rules/archive/refs/heads/main.tar.gz"
)

var installMutex sync.Mutex

type Strategy string

const (
	StrategySequential Strategy = "sequential"
	StrategyParallel   Strategy = "parallel"
	StrategyBoth       Strategy = "both"
)

type DedupeScope string

const (
	DedupeStrict   DedupeScope = "strict"
	DedupeFileLine DedupeScope = "file-line"
)

type ToolKind string

const (
	ToolOpengrep    ToolKind = "opengrep"
	ToolTrivy       ToolKind = "trivy"
	ToolBandit      ToolKind = "bandit"
	ToolBrakeman    ToolKind = "brakeman"
	ToolStaticcheck ToolKind = "staticcheck"
	ToolCpg         ToolKind = "cpg"
	ToolJoern       ToolKind = "joern"
)

type Tool struct {
	Name string
	Kind ToolKind
}

type ToolResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Command    string `json:"command,omitempty"`
	Note       string `json:"note,omitempty"`
	Issues     int    `json:"issues,omitempty"`
}

type StrategyResult struct {
	Name       string       `json:"name"`
	Jobs       int          `json:"jobs"`
	DurationMs int64        `json:"duration_ms"`
	Tools      []ToolResult `json:"tools"`
	Issues     []Issue      `json:"issues,omitempty"`
	RawIssues  int          `json:"issues_raw_count,omitempty"`
}

type Summary struct {
	CodeDir     string           `json:"code_dir"`
	Strategies  []StrategyResult `json:"strategies"`
	FinalIssues []Issue          `json:"final_issues,omitempty"`
}

type Context struct {
	CodeDir        string
	Jobs           int
	RulesDir       string
	RulesAikido    string
	RulesAmplify   string
	RefreshRules   bool
	Verbose        bool
	UseRulesPaths  bool
	DedupeScope    DedupeScope
	InstallMissing bool
}

type Issue struct {
	Analyzer    string           `json:"analyzer"`
	Code        string           `json:"code"`
	File        string           `json:"file"`
	Line        int              `json:"line"`
	Message     string           `json:"message"`
	Fingerprint string           `json:"fingerprint"`
	Enrichment  *IssueEnrichment `json:"enrichment,omitempty"`
}

func main() {
	var (
		codeDir        = flag.String("code-dir", ".", "Path to code directory")
		strategy       = flag.String("strategy", "parallel", "Strategy: sequential|parallel|both")
		jobs           = flag.Int("jobs", 0, "Total jobs (cores)")
		toolsFlag      = flag.String("tools", "", "Comma-separated tools list")
		rulesDir       = flag.String("rules-dir", "./rules", "Rules directory")
		rulesAikido    = flag.String("rules-aikido", "", "Override Aikido rules path")
		rulesAmplify   = flag.String("rules-amplify", "", "Override Amplify rules path")
		noRefresh      = flag.Bool("no-refresh", false, "Skip rules refresh")
		jsonOut        = flag.String("json-out", "", "Write JSON summary to file")
		sarifOut       = flag.String("sarif-out", "", "Write SARIF report to file")
		htmlOut        = flag.String("html-out", "", "Write HTML report to file")
		verbose        = flag.Bool("verbose", false, "Print tool commands")
		dedupeScope    = flag.String("dedupe-scope", "strict", "Dedupe scope: strict|file-line")
		installMissing = flag.Bool("install-missing", true, "Install missing tools when possible")
		llmEnrich      = flag.Bool("llm-enrich", false, "Enrich findings with LLM")
		llmProvider    = flag.String("llm-provider", "openai", "LLM provider name (for metadata/routing)")
		llmModel       = flag.String("llm-model", "", "LLM model name")
		llmEndpoint    = flag.String("llm-endpoint", "", "LLM endpoint URL (OpenAI-compatible chat completions)")
		llmAPIKey      = flag.String("llm-api-key", "", "LLM API key (defaults to OPENAI_API_KEY env var)")
		llmWorkers     = flag.Int("llm-workers", 4, "Concurrent LLM requests")
		llmMaxIssues   = flag.Int("llm-max-issues", 100, "Max issues to enrich with LLM")
		llmTimeoutSec  = flag.Int("llm-timeout", 30, "Timeout in seconds for each LLM call")
	)

	flag.Parse()

	cleanCodeDir := filepath.Clean(*codeDir)
	info, err := os.Stat(cleanCodeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: code-dir %q does not exist: %v\n", *codeDir, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: code-dir %q is not a directory\n", *codeDir)
		os.Exit(1)
	}
	*codeDir = cleanCodeDir

	jobCount := *jobs
	if jobCount <= 0 {
		jobCount = runtime.NumCPU()
	}

	selectedTools := parseTools(*toolsFlag)
	tools := availableTools(selectedTools)
	if len(tools) == 0 {
		fmt.Fprintln(os.Stderr, "No tools selected or available")
		os.Exit(1)
	}

	scope := DedupeScope(strings.ToLower(strings.TrimSpace(*dedupeScope)))
	if scope != DedupeStrict && scope != DedupeFileLine {
		fmt.Fprintln(os.Stderr, "Invalid dedupe-scope:", *dedupeScope)
		os.Exit(1)
	}

	scanContext := Context{
		CodeDir:        *codeDir,
		Jobs:           jobCount,
		RulesDir:       *rulesDir,
		RulesAikido:    *rulesAikido,
		RulesAmplify:   *rulesAmplify,
		RefreshRules:   !*noRefresh,
		Verbose:        *verbose,
		UseRulesPaths:  *rulesAikido != "" || *rulesAmplify != "",
		DedupeScope:    scope,
		InstallMissing: *installMissing,
	}

	var strategies []StrategyResult
	switch Strategy(*strategy) {
	case StrategySequential:
		strategies = append(strategies, runSequential("sequential", tools, scanContext))
	case StrategyParallel:
		strategies = append(strategies, runParallel("parallel", tools, scanContext))
	case StrategyBoth:
		strategies = append(strategies, runSequential("sequential", tools, scanContext))
		strategies = append(strategies, runParallel("parallel", tools, scanContext))
	default:
		fmt.Fprintln(os.Stderr, "Invalid strategy:", *strategy)
		os.Exit(1)
	}

	summary := Summary{
		CodeDir:    scanContext.CodeDir,
		Strategies: strategies,
	}
	finalIssues, _ := dedupeIssues(collectIssues(strategies), scanContext.DedupeScope)
	if *llmEnrich {
		cfg := LLMConfig{
			Provider:  strings.TrimSpace(*llmProvider),
			Model:     strings.TrimSpace(*llmModel),
			Endpoint:  strings.TrimSpace(*llmEndpoint),
			APIKey:    strings.TrimSpace(*llmAPIKey),
			Workers:   *llmWorkers,
			MaxIssues: *llmMaxIssues,
			Timeout:   time.Duration(*llmTimeoutSec) * time.Second,
		}
		if cfg.Endpoint == "" {
			cfg.Endpoint = "https://api.openai.com/v1/chat/completions"
		}
		if cfg.APIKey == "" {
			cfg.APIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
		var err error
		finalIssues, err = enrichIssuesWithLLM(context.Background(), finalIssues, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "LLM enrichment warning:", err)
		}
	}
	summary.FinalIssues = finalIssues

	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "JSON marshal failed:", err)
		os.Exit(1)
	}
	fmt.Println(string(payload))

	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to write json-out:", err)
			os.Exit(1)
		}
	}
	if *sarifOut != "" {
		if err := writeSARIF(*sarifOut, finalIssues); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to write sarif-out:", err)
			os.Exit(1)
		}
	}
	if *htmlOut != "" {
		if err := writeHTML(*htmlOut, finalIssues); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to write html-out:", err)
			os.Exit(1)
		}
	}
}

func collectIssues(strategies []StrategyResult) []Issue {
	var issues []Issue
	for _, strategy := range strategies {
		issues = append(issues, strategy.Issues...)
	}
	return issues
}

func parseTools(raw string) map[string]struct{} {
	selected := make(map[string]struct{})
	if strings.TrimSpace(raw) == "" {
		return selected
	}
	for _, item := range strings.Split(raw, ",") {
		tool := strings.ToLower(strings.TrimSpace(item))
		if tool != "" {
			selected[tool] = struct{}{}
		}
	}
	return selected
}

func availableTools(selected map[string]struct{}) []Tool {
	all := []Tool{
		{Name: "opengrep", Kind: ToolOpengrep},
		{Name: "trivy", Kind: ToolTrivy},
		{Name: "bandit", Kind: ToolBandit},
		{Name: "brakeman", Kind: ToolBrakeman},
		{Name: "gostaticcheck", Kind: ToolStaticcheck},
		{Name: "cpg", Kind: ToolCpg},
		{Name: "joern", Kind: ToolJoern},
	}

	if len(selected) == 0 {
		return all
	}

	var filtered []Tool
	for _, tool := range all {
		if _, ok := selected[strings.ToLower(tool.Name)]; ok {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func runSequential(name string, tools []Tool, context Context) StrategyResult {
	start := time.Now()
	var results []ToolResult
	var issues []Issue
	for _, tool := range tools {
		result, toolIssues := runTool(tool, context)
		results = append(results, result)
		issues = append(issues, toolIssues...)
	}
	deduped, rawCount := dedupeIssues(issues, context.DedupeScope)
	return StrategyResult{
		Name:       name,
		Jobs:       context.Jobs,
		DurationMs: time.Since(start).Milliseconds(),
		Tools:      results,
		Issues:     deduped,
		RawIssues:  rawCount,
	}
}

func runParallel(name string, tools []Tool, context Context) StrategyResult {
	start := time.Now()
	concurrency := context.Jobs
	if concurrency > len(tools) {
		concurrency = len(tools)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	jobsPerTool := context.Jobs / concurrency
	if jobsPerTool < 1 {
		jobsPerTool = 1
	}

	context.Jobs = jobsPerTool

	results := make([]ToolResult, len(tools))
	issuesByTool := make([][]Issue, len(tools))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for idx, tool := range tools {
		wg.Add(1)
		go func(i int, t Tool) {
			defer wg.Done()
			sem <- struct{}{}
			result, toolIssues := runTool(t, context)
			results[i] = result
			issuesByTool[i] = toolIssues
			<-sem
		}(idx, tool)
	}

	wg.Wait()
	var issues []Issue
	for _, toolIssues := range issuesByTool {
		issues = append(issues, toolIssues...)
	}
	deduped, rawCount := dedupeIssues(issues, context.DedupeScope)

	return StrategyResult{
		Name:       name,
		Jobs:       jobsPerTool,
		DurationMs: time.Since(start).Milliseconds(),
		Tools:      results,
		Issues:     deduped,
		RawIssues:  rawCount,
	}
}

func runTool(tool Tool, context Context) (ToolResult, []Issue) {
	start := time.Now()
	cmd, note, err := buildCommand(tool, context)
	if err != nil {
		return ToolResult{
			Name:       tool.Name,
			Status:     "skipped",
			DurationMs: time.Since(start).Milliseconds(),
			Note:       err.Error(),
		}, nil
	}

	if context.Verbose {
		fmt.Printf("Running %s: %s\n", tool.Name, strings.Join(cmd.Args, " "))
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	duration := time.Since(start).Milliseconds()
	outputIssues := parseToolOutput(tool, stdout.Bytes(), stderr.Bytes())

	if err == nil {
		return ToolResult{
			Name:       tool.Name,
			Status:     "ok",
			ExitCode:   intPtr(0),
			DurationMs: duration,
			Command:    strings.Join(cmd.Args, " "),
			Note:       note,
			Issues:     len(outputIssues),
		}, outputIssues
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		return ToolResult{
			Name:       tool.Name,
			Status:     "failed",
			ExitCode:   &code,
			DurationMs: duration,
			Command:    strings.Join(cmd.Args, " "),
			Note:       note,
			Issues:     len(outputIssues),
		}, outputIssues
	}

	return ToolResult{
		Name:       tool.Name,
		Status:     "error",
		DurationMs: duration,
		Command:    strings.Join(cmd.Args, " "),
		Note:       err.Error(),
		Issues:     len(outputIssues),
	}, outputIssues
}

func buildCommand(tool Tool, context Context) (*exec.Cmd, string, error) {
	switch tool.Kind {
	case ToolOpengrep:
		return buildOpengrepCommand(context)
	case ToolTrivy:
		return buildSimpleCommand("trivy", context, func(args *[]string) {
			*args = append(*args, "config", "--format", "json")
			*args = append(*args, context.CodeDir)
		})
	case ToolBandit:
		return buildSimpleCommand("bandit", context, func(args *[]string) {
			*args = append(*args, "-r", context.CodeDir, "-f", "json", "-x", ".git,.betterscan")
		})
	case ToolBrakeman:
		hasRuby, err := hasRubyFiles(context.CodeDir)
		if err != nil {
			return nil, "", err
		}
		isRails, err := isRailsApp(context.CodeDir)
		if err != nil {
			return nil, "", err
		}
		if !hasRuby && !isRails {
			return nil, "", errors.New("Brakeman skipped: no Ruby files or Rails app detected")
		}
		return buildSimpleCommand("brakeman", context, func(args *[]string) {
			*args = append(*args, "-q", "-f", "json", context.CodeDir)
		})
	case ToolStaticcheck:
		if !hasGoMod(context.CodeDir) {
			return nil, "", errors.New("GoStaticcheck skipped: go.mod not found")
		}
		return buildSimpleCommand("staticcheck", context, func(args *[]string) {
			*args = append(*args, "-f", "json", "./...")
		})
	case ToolCpg:
		if err := cpgInstallPreflight(); err != nil {
			return nil, "", err
		}
		hasSources, err := hasSupportedCpgSources(context.CodeDir)
		if err != nil {
			return nil, "", err
		}
		if !hasSources {
			return nil, "", errors.New("Fraunhofer CPG skipped: no supported source files detected")
		}
		return buildCpgCommand(context)
	case ToolJoern:
		return buildJoernCommand(context)
	default:
		return nil, "", errors.New("unsupported tool")
	}
}

func buildSimpleCommand(bin string, context Context, buildArgs func(args *[]string)) (*exec.Cmd, string, error) {
	path, err := exec.LookPath(bin)
	note := ""
	if err != nil {
		customPath := findPreinstalledBin(bin)
		if customPath != "" {
			path = customPath
			err = nil
		} else if context.InstallMissing {
			var installErr error
			note, installErr = installTool(bin)
			if installErr == nil {
				path, err = exec.LookPath(bin)
			} else {
				return nil, "", fmt.Errorf("failed to install %s: %w", bin, installErr)
			}
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("binary not found: %s", bin)
	}
	args := []string{path}
	buildArgs(&args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = context.CodeDir
	return cmd, note, nil
}

func findPreinstalledBin(bin string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var candidates []string
	switch bin {
	case "staticcheck":
		candidates = append(candidates, filepath.Join(home, "go", "bin", "staticcheck"))
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			candidates = append(candidates, filepath.Join(gopath, "bin", "staticcheck"))
		}
	case "bandit":
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "bandit"))
		candidates = append(candidates, filepath.Join(home, "Library", "Python", "3.9", "bin", "bandit"))
	case "brakeman":
		candidates = append(candidates, filepath.Join(home, ".gem", "ruby", "2.6.0", "bin", "brakeman"))
	case "trivy":
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "trivy"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			addPathDir(filepath.Dir(c))
			return c
		}
	}
	return ""
}

func buildOpengrepCommand(context Context) (*exec.Cmd, string, error) {
	opengrepBin := os.Getenv("OPENGREP_BIN")
	if opengrepBin == "" {
		home, _ := os.UserHomeDir()
		defaultPath := filepath.Join(home, ".opengrep/cli/latest/opengrep")
		if _, err := os.Stat(defaultPath); err == nil {
			opengrepBin = defaultPath
		} else {
			var err error
			opengrepBin, err = exec.LookPath("opengrep")
			if err != nil {
				if context.InstallMissing {
					note, installErr := installOpengrep()
					if installErr == nil {
						opengrepBin, err = exec.LookPath("opengrep")
						if err == nil {
							return buildOpengrepCommandWithBin(context, opengrepBin, note)
						}
					}
				}
				return nil, "", errors.New("OpenGrep binary not found")
			}
		}
	}

	return buildOpengrepCommandWithBin(context, opengrepBin, "")
}

func buildOpengrepCommandWithBin(context Context, opengrepBin, note string) (*exec.Cmd, string, error) {
	aikidoPath := context.RulesAikido
	amplifyPath := context.RulesAmplify
	if !context.UseRulesPaths {
		aikidoPath = filepath.Join(context.RulesDir, "aikido")
		amplifyPath = filepath.Join(context.RulesDir, "amplify")
		if context.RefreshRules {
			if err := refreshRules(context.RulesDir); err != nil {
				return nil, "", err
			}
		}
	}

	if err := ensureDir(aikidoPath); err != nil {
		return nil, "", fmt.Errorf("Aikido rules not found at %s", aikidoPath)
	}
	if err := ensureDir(amplifyPath); err != nil {
		return nil, "", fmt.Errorf("Amplify rules not found at %s", amplifyPath)
	}

	aikidoAbs, _ := filepath.Abs(aikidoPath)
	amplifyAbs, _ := filepath.Abs(amplifyPath)

	args := []string{opengrepBin}
	args = append(args,
		"scan",
	)
	if context.Jobs > 0 {
		args = append(args, "-j", fmt.Sprint(context.Jobs))
	}
	args = append(args,
		"-f", aikidoAbs,
		"-f", amplifyAbs,
		"--exclude", ".git/**",
		"--exclude", ".betterscan/**",
		"--no-git-ignore",
		"--json",
		context.CodeDir,
	)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = context.CodeDir
	logPath := filepath.Join(os.TempDir(), "opengrep.log")
	cmd.Env = append(os.Environ(), fmt.Sprintf("SEMGREP_LOG_FILE=%s", logPath))
	return cmd, note, nil
}

func refreshRules(rulesDir string) error {
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return err
	}
	if err := downloadRules("aikido", aikidoRulesURL, rulesDir); err != nil {
		return err
	}
	if err := downloadRules("amplify", amplifyRulesURL, rulesDir); err != nil {
		return err
	}
	return nil
}

func downloadRules(name, url, rulesDir string) error {
	resp, err := httpGetWithRetry(url)
	if err != nil {
		return fmt.Errorf("download %s rules: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("download %s rules: status %s", name, resp.Status)
	}

	tempDir, err := os.MkdirTemp("", "betterscan-rules-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, fmt.Sprintf("%s.tar.gz", name))
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	out.Close()

	if err := extractTarGz(archivePath, tempDir); err != nil {
		return err
	}

	extractedRoot, err := findFirstDir(tempDir)
	if err != nil || extractedRoot == "" {
		return fmt.Errorf("no extracted directory for %s rules", name)
	}

	sourceDir := filepath.Join(extractedRoot, "rules")
	if _, err := os.Stat(sourceDir); err != nil {
		sourceDir = extractedRoot
	}

	targetDir := filepath.Join(rulesDir, name)
	os.RemoveAll(targetDir)

	copied, err := copySemgrepRules(sourceDir, targetDir)
	if err != nil {
		return err
	}
	if copied == 0 {
		return os.Rename(sourceDir, targetDir)
	}

	return nil
}

func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar entry: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

func findFirstDir(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			return filepath.Join(root, entry.Name()), nil
		}
	}
	return "", nil
}

func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func intPtr(v int) *int {
	return &v
}

func copySemgrepRules(sourceDir, targetDir string) (int, error) {
	var copied int
	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "rules:") || !strings.Contains(content, "- id:") {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		dest := filepath.Join(targetDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if _, err := copyFile(path, dest); err != nil {
			return err
		}
		copied++
		return nil
	})
	return copied, err
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, in)
}

func hasGoMod(codeDir string) bool {
	_, err := os.Stat(filepath.Join(codeDir, "go.mod"))
	return err == nil
}

func hasRubyFiles(codeDir string) (bool, error) {
	found := false
	err := filepath.WalkDir(codeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".rb") {
			found = true
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return false, err
	}
	return found, nil
}

func isRailsApp(codeDir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(codeDir, "config", "application.rb")); err == nil {
		return true, nil
	}
	gemfile := filepath.Join(codeDir, "Gemfile")
	data, err := os.ReadFile(gemfile)
	if err != nil {
		return false, nil
	}
	if strings.Contains(string(data), "rails") {
		return true, nil
	}
	return false, nil
}

func parseToolOutput(tool Tool, stdout []byte, stderr []byte) []Issue {
	// Joern emits plain-text "Result:" lines (not JSON). Parse before the
	// empty-JSON short-circuit so stderr-only output is still handled.
	if tool.Kind == ToolJoern {
		return parseJoernScan(stdout, stderr, tool.Name)
	}
	if tool.Kind == ToolCpg {
		payload := cpgJSONFromMixedOutput(stdout, stderr)
		if len(payload) == 0 {
			return nil
		}
		return parseCpg(payload, tool.Name)
	}

	output := selectJSONPayload(stdout, stderr)
	if len(output) == 0 {
		return nil
	}
	switch tool.Kind {
	case ToolOpengrep:
		return parseOpenGrep(output, tool.Name)
	case ToolTrivy:
		return parseTrivy(output, tool.Name)
	case ToolBandit:
		return parseBandit(output, tool.Name)
	case ToolBrakeman:
		return parseBrakeman(output, tool.Name)
	case ToolStaticcheck:
		return parseStaticcheck(output, tool.Name)
	default:
		return nil
	}
}

func selectJSONPayload(stdout []byte, stderr []byte) []byte {
	if bytes.Contains(stdout, []byte("{")) {
		return stdout
	}
	if bytes.Contains(stderr, []byte("{")) {
		return stderr
	}
	if len(stdout) > 0 {
		return stdout
	}
	return stderr
}

func extractJSONBlob(output []byte) []byte {
	start := bytes.IndexByte(output, '{')
	if start == -1 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(output[start:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil
	}
	return raw
}

func parseOpenGrep(output []byte, analyzer string) []Issue {
	var payload struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message string `json:"message"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		if blob := extractJSONBlob(output); blob != nil {
			if err := json.Unmarshal(blob, &payload); err != nil {
				return nil
			}
		} else {
			return nil
		}
	}
	var issues []Issue
	for _, item := range payload.Results {
		issues = append(issues, newIssue(analyzer, item.CheckID, item.Path, item.Start.Line, item.Extra.Message))
	}
	return issues
}

func parseBandit(output []byte, analyzer string) []Issue {
	var payload struct {
		Results []struct {
			TestID    string `json:"test_id"`
			Filename  string `json:"filename"`
			Line      int    `json:"line_number"`
			IssueText string `json:"issue_text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil
	}
	var issues []Issue
	for _, item := range payload.Results {
		issues = append(issues, newIssue(analyzer, item.TestID, item.Filename, item.Line, item.IssueText))
	}
	return issues
}

func parseBrakeman(output []byte, analyzer string) []Issue {
	var payload struct {
		Warnings []struct {
			CheckName string `json:"check_name"`
			File      string `json:"file"`
			Line      int    `json:"line"`
			Message   string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil
	}
	var issues []Issue
	for _, item := range payload.Warnings {
		issues = append(issues, newIssue(analyzer, item.CheckName, item.File, item.Line, item.Message))
	}
	return issues
}

func parseTrivy(output []byte, analyzer string) []Issue {
	var payload struct {
		Results []struct {
			Target            string `json:"Target"`
			Misconfigurations []struct {
				ID          string `json:"ID"`
				Title       string `json:"Title"`
				Description string `json:"Description"`
				Message     string `json:"Message"`
				Location    struct {
					File      string `json:"File"`
					StartLine int    `json:"StartLine"`
				} `json:"Location"`
			} `json:"Misconfigurations"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil
	}
	var issues []Issue
	for _, result := range payload.Results {
		for _, mis := range result.Misconfigurations {
			message := firstNonEmpty(mis.Title, mis.Description, mis.Message)
			file := mis.Location.File
			if file == "" {
				file = result.Target
			}
			issues = append(issues, newIssue(analyzer, mis.ID, file, mis.Location.StartLine, message))
		}
	}
	return issues
}

func parseStaticcheck(output []byte, analyzer string) []Issue {
	lines := bytes.Split(output, []byte("\n"))
	var issues []Issue
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item struct {
			Code     string `json:"code"`
			Message  string `json:"message"`
			Location struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"location"`
		}
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		issues = append(issues, newIssue(analyzer, item.Code, item.Location.File, item.Location.Line, item.Message))
	}
	return issues
}

func newIssue(analyzer, code, file string, line int, message string) Issue {
	fingerprint := hashIssue(analyzer, code, file, line, message)
	return Issue{
		Analyzer:    analyzer,
		Code:        code,
		File:        file,
		Line:        line,
		Message:     message,
		Fingerprint: fingerprint,
	}
}

func hashIssue(analyzer, code, file string, line int, message string) string {
	value := fmt.Sprintf("%s|%s|%s|%d|%s", analyzer, code, file, line, message)
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dedupeIssues(issues []Issue, scope DedupeScope) ([]Issue, int) {
	rawCount := len(issues)
	byKey := make(map[string]Issue)
	for _, issue := range issues {
		key := ""
		switch scope {
		case DedupeFileLine:
			key = fmt.Sprintf("%s|%d", issue.File, issue.Line)
		case DedupeStrict:
			fallthrough
		default:
			key = fmt.Sprintf("%s|%s|%s", issue.Analyzer, issue.Code, issue.Fingerprint)
		}
		existing, ok := byKey[key]
		if !ok || len(issue.Message) > len(existing.Message) {
			byKey[key] = issue
		}
	}
	deduped := make([]Issue, 0, len(byKey))
	for _, issue := range byKey {
		deduped = append(deduped, issue)
	}
	return deduped, rawCount
}

func installTool(bin string) (string, error) {
	switch bin {
	case "opengrep":
		return installOpengrep()
	case "trivy":
		return installTrivy()
	case "bandit":
		return installBandit()
	case "brakeman":
		return installBrakeman()
	case "staticcheck":
		return installStaticcheck()
	case "cpg":
		return installCpgTool()
	case "joern":
		return installJoernScan(Context{InstallMissing: true})
	default:
		return "", fmt.Errorf("no installer for %s", bin)
	}
}

func installOpengrep() (string, error) {
	installMutex.Lock()
	defer installMutex.Unlock()

	if _, err := exec.LookPath("opengrep"); err == nil {
		return "opengrep already installed", nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		defaultPath := filepath.Join(home, ".opengrep/cli/latest/opengrep")
		if _, err := os.Stat(defaultPath); err == nil {
			return "opengrep already installed", nil
		}
	}

	tempDir, err := os.MkdirTemp("", "opengrep-install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "install.sh")
	if err := downloadFile("https://raw.githubusercontent.com/opengrep/opengrep/main/install.sh", scriptPath); err != nil {
		return "", err
	}

	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return "", err
	}

	_, err = runInstallCommand(scriptPath)
	if err != nil {
		return "", err
	}
	return "installed opengrep via install.sh", nil
}

func installTrivy() (string, error) {
	installMutex.Lock()
	defer installMutex.Unlock()

	if _, err := exec.LookPath("trivy"); err == nil {
		return "trivy already installed", nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		dest := filepath.Join(home, ".local", "bin", "trivy")
		if _, err := os.Stat(dest); err == nil {
			return "trivy already installed", nil
		}
	}

	if runtime.GOOS == "darwin" && hasCommand("brew") {
		_, err := runInstallCommand("brew", "install", "trivy")
		if err != nil {
			return "", err
		}
		return "installed trivy via brew", nil
	}
	if hasCommand("sh") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dest := filepath.Join(home, ".local", "bin")
		addPathDir(dest)

		tempDir, err := os.MkdirTemp("", "trivy-install-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tempDir)

		scriptPath := filepath.Join(tempDir, "install.sh")
		if err := downloadFile("https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh", scriptPath); err != nil {
			return "", err
		}

		if err := os.Chmod(scriptPath, 0o755); err != nil {
			return "", err
		}

		_, err = runInstallCommand(scriptPath, "-b", dest)
		if err != nil {
			return "", err
		}
		return "installed trivy via install.sh", nil
	}
	return "", errors.New("Trivy install requires brew or sh")
}

func installBandit() (string, error) {
	installMutex.Lock()
	defer installMutex.Unlock()

	if _, err := exec.LookPath("bandit"); err == nil {
		return "bandit already installed", nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates := []string{
			filepath.Join(home, ".local", "bin", "bandit"),
			filepath.Join(home, "Library", "Python", "3.9", "bin", "bandit"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return "bandit already installed", nil
			}
		}
	}

	addPathDir(filepath.Join(os.Getenv("HOME"), ".local", "bin"))
	addPathDir(filepath.Join(os.Getenv("HOME"), "Library", "Python", "3.9", "bin"))
	if hasCommand("python3") {
		_, err := runInstallCommand("python3", "-m", "pip", "install", "--user", "bandit")
		if err != nil {
			return "", err
		}
		return "installed bandit via python3 -m pip", nil
	}
	if hasCommand("pip") {
		_, err := runInstallCommand("pip", "install", "--user", "bandit")
		if err != nil {
			return "", err
		}
		return "installed bandit via pip", nil
	}
	return "", errors.New("Bandit install requires python3 or pip")
}

func installBrakeman() (string, error) {
	installMutex.Lock()
	defer installMutex.Unlock()

	if _, err := exec.LookPath("brakeman"); err == nil {
		return "brakeman already installed", nil
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		dest := filepath.Join(home, ".gem", "ruby", "2.6.0", "bin", "brakeman")
		if _, err := os.Stat(dest); err == nil {
			return "brakeman already installed", nil
		}
	}

	addPathDir(filepath.Join(os.Getenv("HOME"), ".gem", "ruby", "2.6.0", "bin"))
	if !hasCommand("gem") {
		return "", errors.New("Brakeman install requires gem")
	}
	_, err := runInstallCommand("gem", "install", "brakeman")
	if err == nil {
		return "installed brakeman via gem", nil
	}
	_, err = runInstallCommand("gem", "install", "brakeman", "-v", "5.4.1")
	if err != nil {
		return "", err
	}
	return "installed brakeman via gem (5.4.1)", nil
}

func installStaticcheck() (string, error) {
	installMutex.Lock()
	defer installMutex.Unlock()

	if _, err := exec.LookPath("staticcheck"); err == nil {
		return "staticcheck already installed", nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		dest := filepath.Join(home, "go", "bin", "staticcheck")
		if _, err := os.Stat(dest); err == nil {
			return "staticcheck already installed", nil
		}
	}

	if err == nil {
		addPathDir(filepath.Join(home, "go", "bin"))
	} else {
		addPathDir(filepath.Join(os.Getenv("HOME"), "go", "bin"))
	}
	if !hasCommand("go") {
		return "", errors.New("GoStaticcheck install requires go")
	}
	_, err = runInstallCommand("go", "install", "honnef.co/go/tools/cmd/staticcheck@latest")
	if err != nil {
		// Fallback to v0.5.1
		_, err = runInstallCommand("go", "install", "honnef.co/go/tools/cmd/staticcheck@v0.5.1")
		if err != nil {
			// Fallback to v0.4.7
			_, err = runInstallCommand("go", "install", "honnef.co/go/tools/cmd/staticcheck@v0.4.7")
			if err != nil {
				return "", err
			}
			return "installed staticcheck via go install (v0.4.7 fallback)", nil
		}
		return "installed staticcheck via go install (v0.5.1 fallback)", nil
	}
	return "installed staticcheck via go install", nil
}

func runInstallCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s failed: %s", name, msg)
	}
	return strings.TrimSpace(string(output)), nil
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func addPathDir(dir string) {
	if dir == "" {
		return
	}
	current := os.Getenv("PATH")
	for _, entry := range strings.Split(current, string(os.PathListSeparator)) {
		if entry == dir {
			return
		}
	}
	os.Setenv("PATH", current+string(os.PathListSeparator)+dir)
}

func buildJoernCommand(context Context) (*exec.Cmd, string, error) {
	joernBin := os.Getenv("JOERN_SCAN_BIN")
	if joernBin == "" {
		home, _ := os.UserHomeDir()
		customPath := filepath.Join(home, ".betterscan", "joern", "joern-cli", "joern-scan")
		if _, err := os.Stat(customPath); err == nil {
			joernBin = customPath
		} else {
			var err error
			joernBin, err = exec.LookPath("joern-scan")
			if err != nil {
				if context.InstallMissing {
					note, installErr := installJoernScan(context)
					if installErr == nil {
						if _, err := os.Stat(customPath); err == nil {
							joernBin = customPath
						} else {
							joernBin, err = exec.LookPath("joern-scan")
						}
						if err == nil {
							return buildJoernCommandWithBin(context, joernBin, note)
						}
					}
					if installErr != nil {
						err = installErr
					}
				}
				return nil, "", fmt.Errorf("joern-scan binary not found: %w", err)
			}
		}
	}
	return buildJoernCommandWithBin(context, joernBin, "")
}

func buildJoernCommandWithBin(context Context, joernBin string, note string) (*exec.Cmd, string, error) {
	// Let's ensure Java is available
	java, err := ensureJava(context.InstallMissing, false)
	if err != nil {
		return nil, "", err
	}

	args := []string{joernBin, context.CodeDir}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = context.CodeDir

	cmd.Env = os.Environ()
	if java.JavaHome != "" {
		cmd.Env = append(cmd.Env, "JAVA_HOME="+java.JavaHome)
		javaBinDir := filepath.Dir(java.JavaBin)
		for i, e := range cmd.Env {
			if strings.HasPrefix(e, "PATH=") {
				cmd.Env[i] = "PATH=" + javaBinDir + string(os.PathListSeparator) + e[5:]
				break
			}
		}
	}

	notes := []string{}
	if java.Note != "" {
		notes = append(notes, java.Note)
	}
	if note != "" {
		notes = append(notes, note)
	}
	return cmd, strings.Join(notes, "; "), nil
}

func installJoernScan(context Context) (string, error) {
	// First ensure Java is available without lock to prevent deadlock
	java, err := ensureJava(context.InstallMissing, false)
	if err != nil {
		return "", fmt.Errorf("joern-scan requires Java: %w", err)
	}

	installMutex.Lock()
	defer installMutex.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	installDir := filepath.Join(home, ".betterscan", "joern")
	customPath := filepath.Join(installDir, "joern-cli", "joern-scan")
	if _, err := os.Stat(customPath); err == nil {
		return "joern-scan already installed to " + installDir, nil
	}
	if _, err := exec.LookPath("joern-scan"); err == nil {
		return "joern-scan already installed", nil
	}

	// Create the directory
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", err
	}

	// Download joern-install.sh
	tempDir, err := os.MkdirTemp("", "joern-install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "joern-install.sh")
	if err := downloadFile("https://github.com/joernio/joern/releases/latest/download/joern-install.sh", scriptPath); err != nil {
		return "", err
	}

	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return "", err
	}

	// Run the installer with --install-dir
	cmd := exec.Command(scriptPath, "--install-dir="+installDir)

	// Add our Java to PATH and JAVA_HOME for the installer process
	cmd.Env = os.Environ()
	if java.JavaHome != "" {
		cmd.Env = append(cmd.Env, "JAVA_HOME="+java.JavaHome)
		javaBinDir := filepath.Dir(java.JavaBin)
		for i, e := range cmd.Env {
			if strings.HasPrefix(e, "PATH=") {
				cmd.Env[i] = "PATH=" + javaBinDir + string(os.PathListSeparator) + e[5:]
				break
			}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("joern installer failed: %s", msg)
	}

	return "installed joern via joern-install.sh to " + installDir, nil
}

func parseJoernScan(stdout []byte, stderr []byte, analyzer string) []Issue {
	// Prefer stdout; also scan stderr because joern-scan sometimes logs findings there.
	combined := append(append([]byte{}, stdout...), '\n')
	combined = append(combined, stderr...)
	lines := bytes.Split(combined, []byte("\n"))
	var issues []Issue
	seen := make(map[string]struct{})
	for _, lineBytes := range lines {
		line := strings.TrimSpace(string(lineBytes))
		if !strings.HasPrefix(line, "Result:") {
			continue
		}
		issue := parseJoernLine(line, analyzer)
		if issue == nil {
			continue
		}
		key := issue.Fingerprint
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		issues = append(issues, *issue)
	}
	return issues
}

func parseJoernLine(line string, analyzer string) *Issue {
	line = strings.TrimPrefix(line, "Result:")
	line = strings.TrimSpace(line)

	// Format: $QUERY_SCORE : $QUERY_TITLE: $FILEPATH:$LINE_NUMBER:$FUNCTION_NAME
	idx := strings.Index(line, " : ")
	if idx == -1 {
		return nil
	}
	score := strings.TrimSpace(line[:idx])
	remaining := strings.TrimSpace(line[idx+3:])

	lastColon := strings.LastIndex(remaining, ":")
	if lastColon == -1 {
		return nil
	}
	funcName := strings.TrimSpace(remaining[lastColon+1:])

	rem1 := remaining[:lastColon]
	secondLastColon := strings.LastIndex(rem1, ":")
	if secondLastColon == -1 {
		return nil
	}
	lineNumStr := strings.TrimSpace(rem1[secondLastColon+1:])
	lineNum, err := strconv.Atoi(lineNumStr)
	if err != nil {
		return nil
	}

	rem2 := rem1[:secondLastColon]
	thirdLastColon := strings.LastIndex(rem2, ":")
	if thirdLastColon == -1 {
		return nil
	}
	filePath := strings.TrimSpace(rem2[thirdLastColon+1:])
	title := strings.TrimSpace(rem2[:thirdLastColon])

	message := fmt.Sprintf("[%s] %s in function %s", score, title, funcName)
	issue := newIssue(analyzer, title, filePath, lineNum, message)
	return &issue
}
