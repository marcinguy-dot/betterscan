package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"checkmate-web/backend/models"
)

type ScanJob struct {
	ScanID     string `json:"scan_id"`
	ProjectID  string `json:"project_id"`
	RepoURL    string `json:"repo_url"`
	RepoBranch string `json:"repo_branch"`
	Tools      string `json:"tools"`
	Strategy   string `json:"strategy"`
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
	dsn := getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=checkmate port=5432 sslmode=disable")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func processScanJob(ctx context.Context, db *gorm.DB, rdb *redis.Client, job *ScanJob) error {
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

	// Clone repository
	repoDir, err := cloneRepo(job.RepoURL, job.RepoBranch)
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to clone repo: %v", err))
	}
	defer os.RemoveAll(repoDir)

	// Run go-checkmate scanner
	checkmatePath := getEnv("CHECKMATE_PATH", "../go-checkmate/checkmate-go")
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("scan-%s.json", job.ScanID))

	cmd := exec.Command(checkmatePath,
		"--code-dir", repoDir,
		"--strategy", job.Strategy,
		"--tools", job.Tools,
		"--json-out", outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Scan failed: %v\nOutput: %s", err, string(output)))
	}

	// Parse results
	var summary struct {
		FinalIssues []models.Finding `json:"final_issues"`
	}

	resultData, err := os.ReadFile(outputPath)
	if err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to read scan results: %v", err))
	}

	if err := json.Unmarshal(resultData, &summary); err != nil {
		return updateScanFailed(db, job.ScanID, fmt.Sprintf("Failed to parse scan results: %v", err))
	}

	// Save findings
	for i := range summary.FinalIssues {
		summary.FinalIssues[i].ScanID = job.ScanID
		summary.FinalIssues[i].Severity = inferSeverity(summary.FinalIssues[i].Message)
	}

	if err := db.Create(&summary.FinalIssues).Error; err != nil {
		return fmt.Errorf("failed to save findings: %w", err)
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
		return fmt.Errorf("failed to update scan results: %w", err)
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

func cloneRepo(repoURL, branch string) (string, error) {
	repoDir := filepath.Join(os.TempDir(), fmt.Sprintf("repo-%d", time.Now().UnixNano()))

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, repoURL, repoDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %v\nOutput: %s", err, string(output))
	}

	return repoDir, nil
}

func updateScanFailed(db *gorm.DB, scanID, reason string) error {
	completedAt := time.Now()
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
