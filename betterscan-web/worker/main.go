package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"betterscan-web/worker/models"
)

type ScanJob struct {
	ScanID        string `json:"scan_id"`
	ProjectID     string `json:"project_id"`
	RepoURL       string `json:"repo_url"`
	RepoBranch    string `json:"repo_branch"`
	Tools         string `json:"tools"`
	Strategy      string `json:"strategy"`
	CloneUsername string `json:"clone_username,omitempty"`
	ClonePassword string `json:"clone_password,omitempty"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize database
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("Worker started, waiting for jobs...")

	// Process jobs from Redis queue
	for {
		result, err := rdb.BRPop(ctx, 0*time.Second, "scan:queue").Result()
		if err != nil {
			log.Printf("Error fetching job: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		jobData := result[1]
		var job ScanJob
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			log.Printf("Error unmarshaling job: %v", err)
			continue
		}

		log.Printf("Processing scan job: %s", job.ScanID)
		if err := processScanJob(ctx, db, rdb, &job); err != nil {
			log.Printf("Error processing job %s: %v", job.ScanID, err)
		}
	}
}

func initDB() (*gorm.DB, error) {
	dsn := getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=betterscan port=5432 sslmode=disable")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func processScanJob(ctx context.Context, db *gorm.DB, rdb *redis.Client, job *ScanJob) error {
	// Validate and normalize all job inputs before acting on them. This is the
	// component that runs git/the scanner, so it must not trust the queue.
	if err := validateJob(job); err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Invalid scan job: %v", err))
	}

	// Update scan status to running
	now := time.Now()
	if err := db.Model(&models.Scan{}).
		Where("id = ?", job.ScanID).
		Updates(map[string]interface{}{
			"status":     "running",
			"started_at": &now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update scan status: %w", err)
	}

	// Clone repository (optional HTTPS credentials from VCS adapters)
	repoDir, err := cloneRepo(job.RepoURL, job.RepoBranch, job.CloneUsername, job.ClonePassword)
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to clone repo: %v", err))
	}
	defer os.RemoveAll(repoDir)

	// Run betterscan scanner
	betterscanPath := getEnv("BETTERSCAN_PATH", "../betterscan/betterscan")
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("scan-%s.json", job.ScanID))
	// Persist rules outside the ephemeral repo dir so refreshes can reuse them.
	rulesDir := getEnv("BETTERSCAN_RULES_DIR", filepath.Join(os.TempDir(), "betterscan-rules"))

	args := []string{
		"--code-dir", repoDir,
		"--strategy", job.Strategy,
		"--json-out", outputPath,
		"--rules-dir", rulesDir,
	}
	if job.Tools != "" {
		args = append(args, "--tools", job.Tools)
	}

	cmd := exec.Command(betterscanPath, args...)
	// Ensure installers that drop binaries under ~/.local/bin are found.
	home, _ := os.UserHomeDir()
	pathEnv := os.Getenv("PATH")
	extraPaths := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".opengrep", "cli", "latest"),
		filepath.Join(home, "go", "bin"),
	}
	for _, p := range extraPaths {
		if p != "" && !strings.Contains(pathEnv, p) {
			pathEnv = p + string(os.PathListSeparator) + pathEnv
		}
	}
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		log.Printf("Scan %s scanner output:\n%s", job.ScanID, string(output))
	}
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Scan failed: %v\nOutput: %s", err, string(output)))
	}

	// Parse results. Match betterscan CLI JSON (enrichment is an object or omitted).
	var summary struct {
		Strategies []struct {
			Name   string `json:"name"`
			Tools  []struct {
				Name     string `json:"name"`
				Status   string `json:"status"`
				Note     string `json:"note,omitempty"`
				Issues   int    `json:"issues,omitempty"`
				ExitCode *int   `json:"exit_code,omitempty"`
			} `json:"tools"`
		} `json:"strategies"`
		FinalIssues []models.Finding `json:"final_issues"`
	}

	resultData, err := os.ReadFile(outputPath)
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to read scan results: %v", err))
	}

	if err := json.Unmarshal(resultData, &summary); err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to parse scan results: %v", err))
	}

	// Log per-tool outcomes so skipped/failed plugins are visible in worker logs.
	for _, strat := range summary.Strategies {
		for _, tool := range strat.Tools {
			log.Printf("Scan %s tool %s: status=%s issues=%d note=%s",
				job.ScanID, tool.Name, tool.Status, tool.Issues, tool.Note)
		}
	}

	// Save findings
	scanUUID, err := uuid.Parse(job.ScanID)
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Invalid scan ID: %v", err))
	}

	for i := range summary.FinalIssues {
		summary.FinalIssues[i].ScanID = scanUUID
		summary.FinalIssues[i].Severity = inferSeverity(summary.FinalIssues[i].Message)
		// Normalize invalid empty jsonb payloads to NULL.
		if len(summary.FinalIssues[i].Enrichment) == 0 || string(summary.FinalIssues[i].Enrichment) == "null" {
			summary.FinalIssues[i].Enrichment = nil
		}
	}

	if len(summary.FinalIssues) > 0 {
		if err := db.Create(&summary.FinalIssues).Error; err != nil {
			return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to save findings: %v", err))
		}
	}

	// Update scan with results
	completedAt := time.Now()
	duration := completedAt.Sub(now).Milliseconds()

	var criticalCount, highCount, mediumCount, lowCount int64
	for _, finding := range summary.FinalIssues {
		switch finding.Severity {
		case "critical":
			criticalCount++
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	if err := db.Model(&models.Scan{}).
		Where("id = ?", job.ScanID).
		Updates(map[string]interface{}{
			"status":         "completed",
			"completed_at":   &completedAt,
			"duration_ms":    duration,
			"total_issues":   len(summary.FinalIssues),
			"critical_count": criticalCount,
			"high_count":     highCount,
			"medium_count":   mediumCount,
			"low_count":      lowCount,
		}).Error; err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to update scan results: %v", err))
	}

	// Publish completion event
	event := map[string]interface{}{
		"type":       "scan.completed",
		"scan_id":    job.ScanID,
		"project_id": job.ProjectID,
	}
	eventData, _ := json.Marshal(event)
	rdb.Publish(ctx, "scan:events", eventData)

	log.Printf("Scan %s completed successfully", job.ScanID)
	return nil
}

func cloneRepo(repoURL, branch, cloneUser, clonePass string) (string, error) {
	// Defense-in-depth: re-validate even though the job was validated upstream.
	validURL, err := validateRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	validBranch, err := validateBranch(branch)
	if err != nil {
		return "", err
	}

	cloneURL, err := injectCloneCredentials(validURL, cloneUser, clonePass)
	if err != nil {
		return "", err
	}

	repoDir := filepath.Join(os.TempDir(), fmt.Sprintf("repo-%d", time.Now().UnixNano()))

	// "--" stops the URL from being parsed as an option, and --single-branch
	// limits the clone to the requested branch.
	cmd := exec.Command("git", "clone", "--depth", "1", "--single-branch",
		"--branch", validBranch, "--", cloneURL, repoDir)
	// Disable interactive prompts and restrict git to safe transports so a
	// crafted remote cannot trigger local/ext command execution.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=http:https:ssh:git",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Never echo the credential-bearing URL; redacted message only.
		return "", fmt.Errorf("git clone failed: %v\nOutput: %s", err, redactSecrets(string(output), clonePass))
	}

	return repoDir, nil
}

// injectCloneCredentials rewrites https://host/path to https://user:pass@host/path
// for GitHub (x-access-token), GitLab (oauth2), Bitbucket (x-token-auth), etc.
func injectCloneCredentials(repoURL, user, pass string) (string, error) {
	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)
	if user == "" || pass == "" {
		return repoURL, nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse repo url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("authenticated clone only supports http(s) remotes")
	}
	u.User = url.UserPassword(user, pass)
	return u.String(), nil
}

func redactSecrets(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}

func updateScanFailed(db *gorm.DB, scanID, reason string) error {
	completedAt := time.Now()
	log.Printf("Scan %s failed: %s", scanID, reason)
	return db.Model(&models.Scan{}).
		Where("id = ?", scanID).
		Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": &completedAt,
		}).Error
}

func inferSeverity(message string) string {
	msg := message
	if len(msg) > 100 {
		msg = msg[:100]
	}

	lower := msg
	for i := range lower {
		if lower[i] >= 'A' && lower[i] <= 'Z' {
			lower = lower[:i] + string(lower[i]+32) + lower[i+1:]
		}
	}

	if contains(lower, []string{"critical", "rce", "sql injection", "command injection", "auth bypass"}) {
		return "critical"
	}
	if contains(lower, []string{"high", "xss", "csrf", "injection", "overflow", "buffer"}) {
		return "high"
	}
	if contains(lower, []string{"medium", "warning", "weak", "deprecated"}) {
		return "medium"
	}
	return "low"
}

func contains(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
