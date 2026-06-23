package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	
	"checkmate-web/models"
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

func createProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var project models.Project
		if err := c.ShouldBindJSON(&project); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
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
		id := c.Param("id")
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
		id := c.Param("id")
		var project models.Project
		if err := db.Where("id = ?", id).First(&project).Error; err != nil {
			c.JSON(404, gin.H{"error": "Project not found"})
			return
		}
		if err := c.ShouldBindJSON(&project); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := db.Save(&project).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, project)
	}
}

func deleteProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
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
		projectID := c.Query("project_id")
		var scans []models.Scan
		query := db
		if projectID != "" {
			query = query.Where("project_id = ?", projectID)
		}
		if err := query.Order("created_at DESC").Find(&scans).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, scans)
	}
}

func createScan(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var scan models.Scan
		if err := c.ShouldBindJSON(&scan); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		scan.Status = "pending"
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
		id := c.Param("id")
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
		id := c.Param("id")
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
		severity := c.Query("severity")
		projectID := c.Query("project_id")
		isFalsePositive := c.Query("is_false_positive")
		
		var findings []models.Finding
		query := db.Preload("Scan")
		
		if severity != "" {
			query = query.Where("severity = ?", severity)
		}
		if projectID != "" {
			query = query.Joins("JOIN scans ON findings.scan_id = scans.id").
				Where("scans.project_id = ?", projectID)
		}
		if isFalsePositive != "" {
			query = query.Where("is_false_positive = ?", isFalsePositive == "true")
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
		id := c.Param("id")
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		
		if err := db.Model(&models.Finding{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"is_false_positive": true,
				"false_positive_reason": req.Reason,
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
		projectID := c.Query("project_id")
		var schedules []models.Schedule
		query := db
		if projectID != "" {
			query = query.Where("project_id = ?", projectID)
		}
		if err := query.Find(&schedules).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, schedules)
	}
}

func createSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var schedule models.Schedule
		if err := c.ShouldBindJSON(&schedule); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
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
		id := c.Param("id")
		var schedule models.Schedule
		if err := db.Where("id = ?", id).First(&schedule).Error; err != nil {
			c.JSON(404, gin.H{"error": "Schedule not found"})
			return
		}
		if err := c.ShouldBindJSON(&schedule); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := db.Save(&schedule).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, schedule)
	}
}

func deleteSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
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
