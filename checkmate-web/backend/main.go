package main

import (
	"errors"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"checkmate-web/backend/models"
)

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

	// Auto-migrate schemas
	if err := db.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.Scan{},
		&models.Finding{},
		&models.Schedule{},
		&models.APIKey{},
		&models.Webhook{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API routes
	api := router.Group("/api/v1")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Auth routes
		auth := api.Group("/auth")
		{
			auth.GET("/login", handleLogin)
			auth.GET("/callback", handleCallback)
			auth.GET("/logout", handleLogout)
		}

		// Project routes (protected)
		projects := api.Group("/projects")
		{
			projects.GET("", listProjects(db))
			projects.POST("", createProject(db))
			projects.GET("/:id", getProject(db))
			projects.PUT("/:id", updateProject(db))
			projects.DELETE("/:id", deleteProject(db))
		}

		// Scan routes
		scans := api.Group("/scans")
		{
			scans.GET("", listScans(db))
			scans.POST("", createScan(db))
			scans.GET("/:id", getScan(db))
			scans.GET("/:id/findings", getScanFindings(db))
		}

		// Finding routes
		findings := api.Group("/findings")
		{
			findings.GET("", listFindings(db))
			findings.PUT("/:id/false-positive", markFalsePositive(db))
		}

		// Schedule routes
		schedules := api.Group("/schedules")
		{
			schedules.GET("", listSchedules(db))
			schedules.POST("", createSchedule(db))
			schedules.PUT("/:id", updateSchedule(db))
			schedules.DELETE("/:id", deleteSchedule(db))
		}

		// Dashboard routes
		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/stats", getDashboardStats(db))
			dashboard.GET("/trends", getVulnerabilityTrends(db))
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=checkmate port=5432 sslmode=disable"
	}
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	
	return db, nil
}

// Auth handlers (placeholder - implement OAuth2 flow)
func handleLogin(c *gin.Context) {
	provider := c.Query("provider")
	// Redirect to OAuth provider
	c.JSON(200, gin.H{"message": "OAuth login initiated", "provider": provider})
}

func handleCallback(c *gin.Context) {
	// Handle OAuth callback
	c.JSON(200, gin.H{"message": "OAuth callback handled"})
}

func handleLogout(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Logged out"})
}

// Project handlers
func listProjects(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var projects []models.Project
		if err := db.Find(&projects).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, projects)
	}
}

type projectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoURL     string `json:"repo_url"`
	RepoBranch  string `json:"repo_branch"`
	Language    string `json:"language"`
}

// validate normalizes and validates the request, returning sanitized values.
func (r projectRequest) validate() (name, desc, repoURL, branch, lang string, err error) {
	if name, err = validateName(r.Name); err != nil {
		return
	}
	if desc, err = validateDescription(r.Description); err != nil {
		return
	}
	if repoURL, err = validateRepoURL(r.RepoURL); err != nil {
		return
	}
	if branch, err = validateBranch(r.RepoBranch); err != nil {
		return
	}
	if branch == "" {
		branch = "main"
	}
	if lang, err = validateLanguage(r.Language); err != nil {
		return
	}
	return
}

func createProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req projectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		name, desc, repoURL, branch, lang, err := req.validate()
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		project := models.Project{
			Name:        name,
			Description: desc,
			RepoURL:     repoURL,
			RepoBranch:  branch,
			Language:    lang,
		}
		if err := db.Create(&project).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, project)
	}
}

func getProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var project models.Project
		if err := db.Where("id = ?", id).First(&project).Error; err != nil {
			c.JSON(404, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(200, project)
	}
}

func updateProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var project models.Project
		if err := db.Where("id = ?", id).First(&project).Error; err != nil {
			c.JSON(404, gin.H{"error": "Project not found"})
			return
		}
		var req projectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		name, desc, repoURL, branch, lang, err := req.validate()
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// Update only client-controllable fields; never id/created_by/timestamps.
		if err := db.Model(&project).Updates(map[string]interface{}{
			"name":        name,
			"description": desc,
			"repo_url":    repoURL,
			"repo_branch": branch,
			"language":    lang,
		}).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, project)
	}
}

func deleteProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := db.Delete(&models.Project{}, "id = ?", id).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// Scan handlers
func listScans(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db
		if raw := c.Query("project_id"); raw != "" {
			projectID, err := validateUUID(raw)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid project_id"})
				return
			}
			query = query.Where("project_id = ?", projectID)
		}
		var scans []models.Scan
		if err := query.Order("created_at DESC").Find(&scans).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, scans)
	}
}

type scanRequest struct {
	ProjectID string `json:"project_id"`
	Strategy  string `json:"strategy"`
	Tools     string `json:"tools"`
}

func createScan(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req scanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		projectID, err := validateUUID(req.ProjectID)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid project_id"})
			return
		}
		strategy, err := validateStrategy(req.Strategy)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		tools, err := normalizeTools(req.Tools)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// Ensure the referenced project exists before queueing work for it.
		var project models.Project
		if err := db.Where("id = ?", projectID).First(&project).Error; err != nil {
			c.JSON(404, gin.H{"error": "Project not found"})
			return
		}
		scan := models.Scan{
			ProjectID: projectID,
			Status:    "pending",
			Strategy:  strategy,
			Tools:     tools,
		}
		if err := db.Create(&scan).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// TODO: Queue scan job for worker
		c.JSON(201, scan)
	}
}

func getScan(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var scan models.Scan
		if err := db.Preload("Findings").Where("id = ?", id).First(&scan).Error; err != nil {
			c.JSON(404, gin.H{"error": "Scan not found"})
			return
		}
		c.JSON(200, scan)
	}
}

func getScanFindings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var findings []models.Finding
		if err := db.Where("scan_id = ?", id).Find(&findings).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, findings)
	}
}

// Finding handlers
func listFindings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		severity, err := validateSeverity(c.Query("severity"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		fp, fpSet, err := validateBool(c.Query("is_false_positive"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid is_false_positive"})
			return
		}

		var findings []models.Finding
		query := db.Preload("Scan")

		if severity != "" {
			query = query.Where("severity = ?", severity)
		}
		if raw := c.Query("project_id"); raw != "" {
			projectID, err := validateUUID(raw)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid project_id"})
				return
			}
			query = query.Joins("JOIN scans ON findings.scan_id = scans.id").
				Where("scans.project_id = ?", projectID)
		}
		if fpSet {
			query = query.Where("is_false_positive = ?", fp)
		}
		
		if err := query.Order("created_at DESC").Find(&findings).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, findings)
	}
}

func markFalsePositive(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		reason, err := validateReason(req.Reason)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if err := db.Model(&models.Finding{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"is_false_positive": true,
				"false_positive_reason": reason,
			}).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "Marked as false positive"})
	}
}

// Schedule handlers
func listSchedules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db
		if raw := c.Query("project_id"); raw != "" {
			projectID, err := validateUUID(raw)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid project_id"})
				return
			}
			query = query.Where("project_id = ?", projectID)
		}
		var schedules []models.Schedule
		if err := query.Find(&schedules).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, schedules)
	}
}

type scheduleRequest struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	CronExpr  string `json:"cron_expr"`
	Tools     string `json:"tools"`
	Strategy  string `json:"strategy"`
	Enabled   *bool  `json:"enabled"`
}

type scheduleFields struct {
	projectID uuid.UUID
	name      string
	cronExpr  string
	tools     string
	strategy  string
	enabled   bool
}

func (r scheduleRequest) validate() (scheduleFields, error) {
	var f scheduleFields
	var err error
	if f.projectID, err = validateUUID(r.ProjectID); err != nil {
		return f, errors.New("invalid project_id")
	}
	if f.name, err = validateName(r.Name); err != nil {
		return f, err
	}
	if f.cronExpr, err = validateCronExpr(r.CronExpr); err != nil {
		return f, err
	}
	if f.tools, err = normalizeTools(r.Tools); err != nil {
		return f, err
	}
	if f.strategy, err = validateStrategy(r.Strategy); err != nil {
		return f, err
	}
	f.enabled = true
	if r.Enabled != nil {
		f.enabled = *r.Enabled
	}
	return f, nil
}

func createSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req scheduleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		f, err := req.validate()
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var project models.Project
		if err := db.Where("id = ?", f.projectID).First(&project).Error; err != nil {
			c.JSON(404, gin.H{"error": "Project not found"})
			return
		}
		schedule := models.Schedule{
			ProjectID: f.projectID,
			Name:      f.name,
			CronExpr:  f.cronExpr,
			Tools:     f.tools,
			Strategy:  f.strategy,
			Enabled:   f.enabled,
		}
		if err := db.Create(&schedule).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, schedule)
	}
}

func updateSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var schedule models.Schedule
		if err := db.Where("id = ?", id).First(&schedule).Error; err != nil {
			c.JSON(404, gin.H{"error": "Schedule not found"})
			return
		}
		var req scheduleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		f, err := req.validate()
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// Update only client-controllable fields; never id/timestamps.
		if err := db.Model(&schedule).Updates(map[string]interface{}{
			"project_id": f.projectID,
			"name":       f.name,
			"cron_expr":  f.cronExpr,
			"tools":      f.tools,
			"strategy":   f.strategy,
			"enabled":    f.enabled,
		}).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, schedule)
	}
}

func deleteSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := validateUUID(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := db.Delete(&models.Schedule{}, "id = ?", id).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// Dashboard handlers
func getDashboardStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stats struct {
			TotalProjects    int64 `json:"total_projects"`
			TotalScans       int64 `json:"total_scans"`
			RunningScans     int64 `json:"running_scans"`
			TotalFindings    int64 `json:"total_findings"`
			CriticalCount    int64 `json:"critical_count"`
			HighCount        int64 `json:"high_count"`
			MediumCount      int64 `json:"medium_count"`
			LowCount         int64 `json:"low_count"`
		}
		
		db.Model(&models.Project{}).Count(&stats.TotalProjects)
		db.Model(&models.Scan{}).Count(&stats.TotalScans)
		db.Model(&models.Scan{}).Where("status = ?", "running").Count(&stats.RunningScans)
		db.Model(&models.Finding{}).Where("is_false_positive = ?", false).Count(&stats.TotalFindings)
		db.Model(&models.Finding{}).Where("severity = ? AND is_false_positive = ?", "critical", false).Count(&stats.CriticalCount)
		db.Model(&models.Finding{}).Where("severity = ? AND is_false_positive = ?", "high", false).Count(&stats.HighCount)
		db.Model(&models.Finding{}).Where("severity = ? AND is_false_positive = ?", "medium", false).Count(&stats.MediumCount)
		db.Model(&models.Finding{}).Where("severity = ? AND is_false_positive = ?", "low", false).Count(&stats.LowCount)
		
		c.JSON(200, stats)
	}
}

func getVulnerabilityTrends(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Return vulnerability trends over time
		// Group by date and severity
		type TrendData struct {
			Date     string `json:"date"`
			Severity string `json:"severity"`
			Count    int64  `json:"count"`
		}
		
		var trends []TrendData
		db.Model(&models.Finding{}).
			Select("DATE(created_at) as date, severity, COUNT(*) as count").
			Where("is_false_positive = ?", false).
			Group("DATE(created_at), severity").
			Order("date DESC").
			Scan(&trends)
		
		c.JSON(200, trends)
	}
}
