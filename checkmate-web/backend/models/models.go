package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"` // bcrypt hash for local auth
	Provider     string    `gorm:"not null" json:"provider"`      // local, google, github, etc.
	ProviderID   string    `gorm:"not null" json:"provider_id"`
	Role         string    `gorm:"default:user" json:"role"` // admin, user
	LastLoginAt  time.Time `json:"last_login_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Project represents a code project to scan
type Project struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	RepoURL     string    `json:"repo_url"`
	RepoBranch  string    `gorm:"default:main" json:"repo_branch"`
	Language    string    `json:"language"` // go, python, java, etc.
	// Optional link to a VCS connection (GitHub App, GitLab PAT, etc.) for private clone.
	VcsConnectionID *uuid.UUID `gorm:"type:uuid;index" json:"vcs_connection_id,omitempty"`
	RepoExternalID  string     `json:"repo_external_id,omitempty"`
	RepoFullName    string     `json:"repo_full_name,omitempty"`
	CreatedBy       uuid.UUID  `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Scans         []Scan         `gorm:"foreignKey:ProjectID" json:"scans,omitempty"`
	Schedules     []Schedule     `gorm:"foreignKey:ProjectID" json:"schedules,omitempty"`
	VcsConnection *VcsConnection `gorm:"foreignKey:VcsConnectionID" json:"vcs_connection,omitempty"`
}

// VcsConnection links a user to a git host with credentials for listing repos and cloning.
// Secrets are stored encrypted (SecretEnc); never expose them in JSON.
type VcsConnection struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Provider     string    `gorm:"not null;index" json:"provider"`   // github | gitlab | bitbucket | generic
	AuthType     string    `gorm:"not null" json:"auth_type"`        // github_app | pat | oauth
	Host         string    `gorm:"not null" json:"host"`             // github.com, gitlab.com, ...
	ExternalID   string    `gorm:"index" json:"external_id"`         // installation id, etc.
	DisplayName  string    `json:"display_name"`
	SecretEnc    string    `gorm:"type:text" json:"-"`               // encrypted PAT / refresh token / PEM material
	CloneUser    string    `json:"clone_user,omitempty"`             // default HTTPS username for this connection
	MetadataJSON string    `gorm:"type:text" json:"metadata,omitempty"`
	CreatedBy    uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Scan represents a security scan run
type Scan struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID    uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Status       string    `gorm:"not null;index" json:"status"` // pending, running, completed, failed
	Strategy     string    `json:"strategy"` // sequential, parallel, both
	Tools        string    `json:"tools"` // comma-separated list of tools
	DurationMs   int64     `json:"duration_ms"`
	TotalIssues  int       `json:"total_issues"`
	CriticalCount int      `json:"critical_count"`
	HighCount    int       `json:"high_count"`
	MediumCount  int       `json:"medium_count"`
	LowCount     int       `json:"low_count"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	
	// Relations
	Project      Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Findings     []Finding `gorm:"foreignKey:ScanID" json:"findings,omitempty"`
}

// Finding represents a vulnerability finding
type Finding struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ScanID       uuid.UUID `gorm:"type:uuid;not null;index" json:"scan_id"`
	Analyzer     string    `gorm:"index" json:"analyzer"`
	Code         string    `gorm:"index" json:"code"`
	File         string    `json:"file"`
	Line         int       `json:"line"`
	Message      string    `gorm:"type:text" json:"message"`
	Severity     string    `gorm:"index" json:"severity"` // critical, high, medium, low
	Confidence   string    `json:"confidence"` // high, medium, low
	Fingerprint  string    `gorm:"index" json:"fingerprint"`
	IsFalsePositive bool `gorm:"default:false;index" json:"is_false_positive"`
	FalsePositiveReason string `gorm:"type:text" json:"false_positive_reason,omitempty"`
	Enrichment   string    `gorm:"type:jsonb" json:"enrichment,omitempty"` // JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	
	// Relations
	Scan         Scan      `gorm:"foreignKey:ScanID" json:"scan,omitempty"`
}

// Schedule represents a scheduled scan
type Schedule struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Name        string    `gorm:"not null" json:"name"`
	CronExpr    string    `gorm:"not null" json:"cron_expr"` // cron expression
	Tools       string    `json:"tools"`
	Strategy    string    `json:"strategy"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	LastRunAt   *time.Time `json:"last_run_at"`
	NextRunAt   *time.Time `json:"next_run_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// Relations
	Project     Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// APIKey represents an API key for CI/CD integration
type APIKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	KeyHash     string    `gorm:"not null;uniqueIndex" json:"-"`
	KeyPrefix   string    `gorm:"not null" json:"key_prefix"` // First 8 chars for display
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	URL         string    `gorm:"not null" json:"url"`
	Secret      string    `json:"secret"`
	Events      string    `gorm:"not null" json:"events"` // comma-separated: scan.completed, scan.failed
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// Relations
	Project     Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}
