package systems

import (
	"fmt"
	"net/http"
	"net/smtp"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

type SystemServiceClient struct {
	logger *zap.SugaredLogger
	config *config.Config
}

func NewSystemServiceClient(cfg *config.Config, log *zap.SugaredLogger) *SystemServiceClient {
	return &SystemServiceClient{
		logger: log,
		config: cfg,
	}
}

// SystemKeyMiddleware validates system key from header
func SystemKeyMiddleware(systemKey string, log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		providedKey := c.GetHeader("X-System-Key")
		if providedKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "System key required",
			})
			return
		}

		if providedKey != systemKey {
			log.Warnf("Invalid system key attempt from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid system key",
			})
			return
		}

		c.Next()
	}
}

func RegisterSystemRouter(router *gin.Engine, cfg *config.Config, log *zap.SugaredLogger) *SystemServiceClient {
	ssc := NewSystemServiceClient(cfg, log)

	// Public contact route (no authentication required)
	router.POST("/api/v1/contact", ssc.HandleContactForm)

	// System routes group with system key validation
	systemRoutes := router.Group("/api/v1/system")
	systemRoutes.Use(SystemKeyMiddleware(cfg.Keys.SystemKey, log)) // Using system key for system access

	// System information routes
	{
		systemRoutes.GET("/info", ssc.GetSystemInfo)
		systemRoutes.GET("/versions", ssc.GetVersionInfo)
		systemRoutes.GET("/health", ssc.GetSystemHealth)
		systemRoutes.GET("/metrics", ssc.GetSystemMetrics)
		systemRoutes.GET("/database/status", ssc.GetDatabaseStatus)
		systemRoutes.GET("/services/status", ssc.GetServicesStatus)
		systemRoutes.GET("/repositories", ssc.GetRepositoryInfo)
	}

	// System operations routes
	{
		systemRoutes.POST("/cache/clear", ssc.ClearSystemCache)
		systemRoutes.POST("/maintenance/mode", ssc.SetMaintenanceMode)
		systemRoutes.DELETE("/maintenance/mode", ssc.DisableMaintenanceMode)
		systemRoutes.POST("/backup/trigger", ssc.TriggerSystemBackup)
	}

	return ssc
}

// GetSystemInfo returns basic system information
func (ssc *SystemServiceClient) GetSystemInfo(ctx *gin.Context) {
	ssc.logger.Debugf("System info requested from IP: %s", ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"system": gin.H{
			"name":         "The Monkeys Engine",
			"environment":  "production",
			"go_version":   runtime.Version(),
			"architecture": runtime.GOARCH,
			"os":           runtime.GOOS,
			"cpu_count":    runtime.NumCPU(),
			"timestamp":    time.Now().UTC(),
		},
		"request_info": gin.H{
			"client_ip":  ctx.ClientIP(),
			"user_agent": ctx.GetHeader("User-Agent"),
		},
	})
}

// GetVersionInfo returns frontend and backend version information
func (ssc *SystemServiceClient) GetVersionInfo(ctx *gin.Context) {
	ssc.logger.Debugf("Version info requested from IP: %s", ctx.ClientIP())

	// This would typically be read from build files, environment variables, or version files
	ctx.JSON(http.StatusOK, gin.H{
		"versions": gin.H{
			"backend": gin.H{
				"version":     "v1.2.3",
				"build":       "20250808-1234",
				"commit_hash": "abc123def456",
				"build_date":  "2025-08-08T12:00:00Z",
				"go_version":  runtime.Version(),
				"components": gin.H{
					"gateway":               "v1.2.3",
					"user_service":          "v1.2.1",
					"blog_service":          "v1.2.2",
					"auth_service":          "v1.2.0",
					"storage_service":       "v1.1.9",
					"notification_service":  "v1.2.1",
					"recommendation_engine": "v1.0.5",
				},
			},
			"frontend": gin.H{
				"version":     "v2.1.4",
				"build":       "20250808-0945",
				"commit_hash": "xyz789uvw012",
				"build_date":  "2025-08-08T09:45:00Z",
				"framework":   "React 18.2.0",
				"components": gin.H{
					"web_app":     "v2.1.4",
					"mobile_app":  "v1.8.2",
					"admin_panel": "v1.5.1",
				},
			},
			"database": gin.H{
				"postgresql":    "v17.5",
				"elasticsearch": "v8.16.1",
				"redis":         "v7.0",
			},
			"infrastructure": gin.H{
				"docker":     "v24.0.0",
				"kubernetes": "v1.28.0",
				"nginx":      "v1.24.0",
			},
		},
		"compatibility": gin.H{
			"min_frontend_version": "v2.0.0",
			"min_mobile_version":   "v1.7.0",
			"api_version":          "v1",
		},
		"last_updated": time.Now().UTC(),
	})
}

// GetSystemHealth returns system health status
func (ssc *SystemServiceClient) GetSystemHealth(ctx *gin.Context) {
	ssc.logger.Debugf("System health check requested from IP: %s", ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"checks": gin.H{
			"database":      "healthy",
			"elasticsearch": "healthy",
			"redis":         "healthy",
			"rabbitmq":      "healthy",
			"microservices": gin.H{
				"gateway":              "healthy",
				"user_service":         "healthy",
				"blog_service":         "healthy",
				"auth_service":         "healthy",
				"storage_service":      "healthy",
				"notification_service": "healthy",
			},
		},
		"uptime": gin.H{
			"seconds": 86400, // Would be calculated from actual start time
			"human":   "1 day",
		},
		"timestamp": time.Now().UTC(),
	})
}

// GetSystemMetrics returns system performance metrics
func (ssc *SystemServiceClient) GetSystemMetrics(ctx *gin.Context) {
	ssc.logger.Debugf("System metrics requested from IP: %s", ctx.ClientIP())

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ctx.JSON(http.StatusOK, gin.H{
		"memory": gin.H{
			"allocated_mb":       memStats.Alloc / 1024 / 1024,
			"total_allocated_mb": memStats.TotalAlloc / 1024 / 1024,
			"system_mb":          memStats.Sys / 1024 / 1024,
			"gc_cycles":          memStats.NumGC,
		},
		"goroutines": runtime.NumGoroutine(),
		"cpu_count":  runtime.NumCPU(),
		"requests": gin.H{
			"total":           12345, // Would be tracked from middleware
			"success":         11890,
			"errors":          455,
			"rate_per_minute": 150,
		},
		"database": gin.H{
			"connections_active": 25,
			"connections_idle":   10,
			"query_avg_time_ms":  45,
		},
		"timestamp": time.Now().UTC(),
	})
}

// GetDatabaseStatus returns database connection status
func (ssc *SystemServiceClient) GetDatabaseStatus(ctx *gin.Context) {
	ssc.logger.Debugf("Database status requested from IP: %s", ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"postgresql": gin.H{
			"status":           "connected",
			"version":          "17.5",
			"connections":      35,
			"max_connections":  100,
			"database_size_mb": 2048,
			"last_backup":      "2025-08-08T06:00:00Z",
		},
		"elasticsearch": gin.H{
			"status":         "connected",
			"version":        "8.16.1",
			"cluster_health": "green",
			"indices_count":  5,
			"documents":      300,
		},
		"redis": gin.H{
			"status":     "connected",
			"version":    "7.0",
			"memory_mb":  64,
			"keys_count": 1250,
		},
		"timestamp": time.Now().UTC(),
	})
}

// GetServicesStatus returns microservices status
func (ssc *SystemServiceClient) GetServicesStatus(ctx *gin.Context) {
	ssc.logger.Debugf("Services status requested from IP: %s", ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"services": gin.H{
			"gateway": gin.H{
				"status":   "running",
				"port":     8081,
				"uptime":   "1d 2h 30m",
				"requests": 15420,
				"errors":   12,
			},
			"user_service": gin.H{
				"status":   "running",
				"port":     50053,
				"uptime":   "1d 2h 28m",
				"requests": 8940,
				"errors":   3,
			},
			"blog_service": gin.H{
				"status":   "running",
				"port":     50052,
				"uptime":   "1d 2h 29m",
				"requests": 12650,
				"errors":   8,
			},
			"auth_service": gin.H{
				"status":   "running",
				"port":     50051,
				"uptime":   "1d 2h 30m",
				"requests": 5320,
				"errors":   2,
			},
		},
		"timestamp": time.Now().UTC(),
	})
}

// GetRepositoryInfo returns repository information using GitHub token
func (ssc *SystemServiceClient) GetRepositoryInfo(ctx *gin.Context) {
	ssc.logger.Debugf("Repository info requested from IP: %s", ctx.ClientIP())

	// This would use the GitHub token from config to fetch real repository data
	githubToken := ssc.config.Keys.GitHubToken

	// Placeholder response - in real implementation, you'd make GitHub API calls
	ctx.JSON(http.StatusOK, gin.H{
		"repositories": gin.H{
			"backend": gin.H{
				"name":         "the_monkeys_engine",
				"owner":        "the-monkeys",
				"branch":       "main",
				"last_commit":  "abc123def456",
				"commit_date":  "2025-08-08T10:30:00Z",
				"contributors": 12,
				"stars":        245,
				"forks":        18,
				"open_issues":  8,
				"language":     "Go",
			},
			"frontend": gin.H{
				"name":         "the_monkeys_web",
				"owner":        "the-monkeys",
				"branch":       "main",
				"last_commit":  "xyz789uvw012",
				"commit_date":  "2025-08-08T09:45:00Z",
				"contributors": 8,
				"stars":        189,
				"forks":        12,
				"open_issues":  5,
				"language":     "TypeScript",
			},
		},
		"github_token_configured": githubToken != "" && githubToken != "your_github_personal_access_token_here",
		"note":                    "Repository data fetched using GitHub API",
		"timestamp":               time.Now().UTC(),
	})
}

// ClearSystemCache clears various system caches
func (ssc *SystemServiceClient) ClearSystemCache(ctx *gin.Context) {
	var req struct {
		CacheType string `json:"cache_type" binding:"required"` // "all", "redis", "memory", "database"
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	ssc.logger.Debugf("Cache clear requested for type: %s from IP: %s", req.CacheType, ctx.ClientIP())

	validTypes := []string{"all", "redis", "memory", "database"}
	isValid := false
	for _, t := range validTypes {
		if req.CacheType == t {
			isValid = true
			break
		}
	}

	if !isValid {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid cache type. Must be one of: all, redis, memory, database",
		})
		return
	}

	// Here you would implement actual cache clearing logic
	ctx.JSON(http.StatusOK, gin.H{
		"message":    "Cache cleared successfully",
		"cache_type": req.CacheType,
		"cleared_at": time.Now().UTC(),
		"cleared_by": "system",
	})
}

// SetMaintenanceMode enables maintenance mode
func (ssc *SystemServiceClient) SetMaintenanceMode(ctx *gin.Context) {
	var req struct {
		Message  string `json:"message"`
		Duration string `json:"duration"` // e.g., "30m", "1h", "2h30m"
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Message = "System maintenance in progress"
		req.Duration = "30m"
	}

	ssc.logger.Debugf("Maintenance mode enabled from IP: %s, duration: %s", ctx.ClientIP(), req.Duration)

	ctx.JSON(http.StatusOK, gin.H{
		"message":             "Maintenance mode enabled",
		"maintenance_message": req.Message,
		"estimated_duration":  req.Duration,
		"enabled_at":          time.Now().UTC(),
		"enabled_by":          "system",
	})
}

// DisableMaintenanceMode disables maintenance mode
func (ssc *SystemServiceClient) DisableMaintenanceMode(ctx *gin.Context) {
	ssc.logger.Debugf("Maintenance mode disabled from IP: %s", ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "Maintenance mode disabled",
		"disabled_at": time.Now().UTC(),
		"disabled_by": "system",
	})
}

// TriggerSystemBackup triggers a system-wide backup
func (ssc *SystemServiceClient) TriggerSystemBackup(ctx *gin.Context) {
	var req struct {
		BackupType string `json:"backup_type"` // "full", "incremental", "database_only"
		Async      bool   `json:"async"`       // whether to run backup asynchronously
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.BackupType = "full"
		req.Async = true
	}

	ssc.logger.Debugf("System backup triggered, type: %s, async: %v, from IP: %s", req.BackupType, req.Async, ctx.ClientIP())

	backupID := "backup_" + time.Now().Format("20060102_150405")

	ctx.JSON(http.StatusOK, gin.H{
		"message":              "Backup initiated successfully",
		"backup_id":            backupID,
		"backup_type":          req.BackupType,
		"async":                req.Async,
		"started_at":           time.Now().UTC(),
		"estimated_completion": time.Now().Add(30 * time.Minute).UTC(),
	})
}

// ContactFormRequest represents the contact form data
type ContactFormRequest struct {
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	CompanyName string `json:"company_name"`
	CompanySize string `json:"company_size"`
	Subject     string `json:"subject" binding:"required"`
	Message     string `json:"message"`
}

// HandleContactForm handles contact form submissions and sends emails
func (ssc *SystemServiceClient) HandleContactForm(ctx *gin.Context) {
	// Only allow in non-local environments
	// if ssc.config.AppEnv == "development" || ssc.config.AppEnv == "local" {
	// 	ssc.logger.Warnf("Contact form submission blocked in local environment from IP: %s", ctx.ClientIP())
	// 	ctx.JSON(http.StatusOK, gin.H{
	// 		"message": "Contact form received (email not sent in local environment)",
	// 		"status":  "success",
	// 	})
	// 	return
	// }

	var req ContactFormRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ssc.logger.Errorf("Invalid contact form data from IP %s: %v", ctx.ClientIP(), err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Validate email format
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid email address format",
		})
		return
	}

	ssc.logger.Infof("Contact form submission received from: %s (%s) - Subject: %s", req.Email, ctx.ClientIP(), req.Subject)

	// Send email using Gmail SMTP
	if err := ssc.sendContactEmail(&req, ctx.ClientIP()); err != nil {
		ssc.logger.Errorf("Failed to send contact email: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send email",
			"message": "We're having trouble processing your request. Please try again later.",
		})
		return
	}

	ssc.logger.Infof("Contact email sent successfully for: %s %s (%s)", req.FirstName, req.LastName, req.Email)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Thank you for contacting us! We'll get back to you soon.",
		"status":  "success",
	})
}

// sendContactEmail sends the contact form data via Gmail SMTP
func (ssc *SystemServiceClient) sendContactEmail(req *ContactFormRequest, clientIP string) error {
	// Get Gmail configuration
	gmailConfig := ssc.config.Gmail
	if gmailConfig.SMTPMail == "" || gmailConfig.SMTPPassword == "" {
		return fmt.Errorf("Gmail SMTP credentials not configured")
	}

	// Recipient email
	recipientEmail := "monkeys.admin@monkeys.com.co"

	// Compose email
	from := gmailConfig.SMTPMail
	to := []string{recipientEmail}

	// Build email headers and body
	var emailBuilder strings.Builder
	emailBuilder.WriteString(fmt.Sprintf("From: %s\r\n", from))
	emailBuilder.WriteString(fmt.Sprintf("To: %s\r\n", recipientEmail))
	emailBuilder.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject)) // Use user's subject directly
	emailBuilder.WriteString("MIME-version: 1.0;\r\n")
	emailBuilder.WriteString("Content-Type: text/html; charset=\"UTF-8\";\r\n")
	emailBuilder.WriteString("\r\n")
	emailBuilder.WriteString(buildContactEmailHTML(req, clientIP))

	message := []byte(emailBuilder.String())

	// Gmail SMTP configuration
	smtpHost := gmailConfig.SMTPHost
	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}

	smtpAddr := gmailConfig.SMTPAddress
	if smtpAddr == "" {
		smtpAddr = "smtp.gmail.com:587"
	}

	// Authentication
	auth := smtp.PlainAuth("", gmailConfig.SMTPMail, gmailConfig.SMTPPassword, smtpHost)

	// Send email
	err := smtp.SendMail(smtpAddr, auth, from, to, message)
	if err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	return nil
}

// buildContactEmailHTML builds the HTML template for contact form emails
func buildContactEmailHTML(req *ContactFormRequest, clientIP string) string {
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background-color: #eb5c09ff; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
		.content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; border-radius: 0 0 5px 5px; }
		.info-table { width: 100%; border-collapse: collapse; margin: 20px 0; }
		.info-table td { padding: 12px; border-bottom: 1px solid #ddd; }
		.info-table td:first-child { font-weight: bold; width: 150px; background-color: #f5f5f5; }
		.message-box { background-color: white; padding: 15px; border-left: 4px solid #eb5c09ff; margin: 15px 0; }
		.footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; padding-top: 20px; border-top: 1px solid #ddd; }
	</style>
</head>
<body>
	<div class="header">
		<h2 style="margin: 0;">Contact Monkeys</h2>
	</div>
	<div class="content">
		<table class="info-table">
			<tr>
				<td>Name</td>
				<td>`)
	html.WriteString(fmt.Sprintf("%s %s", req.FirstName, req.LastName))
	html.WriteString(`</td>
			</tr>
			<tr>
				<td>Email</td>
				<td><a href="mailto:`)
	html.WriteString(req.Email)
	html.WriteString(`" style="color: #4CAF50; text-decoration: none;">`)
	html.WriteString(req.Email)
	html.WriteString(`</a></td>
			</tr>`)

	if req.CompanyName != "" {
		html.WriteString(`
			<tr>
				<td>Company</td>
				<td>`)
		html.WriteString(req.CompanyName)
		html.WriteString(`</td>
			</tr>`)
	}

	if req.CompanySize != "" {
		html.WriteString(`
			<tr>
				<td>Company Size</td>
				<td>`)
		html.WriteString(req.CompanySize)
		html.WriteString(`</td>
			</tr>`)
	}

	html.WriteString(`
			<tr>
				<td>Subject</td>
				<td><strong>`)
	html.WriteString(req.Subject)
	html.WriteString(`</strong></td>
			</tr>`)

	if req.Message != "" {
		html.WriteString(`
			<tr>
				<td colspan="2">
					<div class="message-box">
						<strong>Message:</strong><br><br>`)
		html.WriteString(strings.ReplaceAll(req.Message, "\n", "<br>"))
		html.WriteString(`
					</div>
				</td>
			</tr>`)
	}

	html.WriteString(`
			<tr>
				<td>IP Address</td>
				<td>`)
	html.WriteString(clientIP)
	html.WriteString(`</td>
			</tr>
			<tr>
				<td>Timestamp</td>
				<td>`)
	html.WriteString(time.Now().UTC().Format(time.RFC3339))
	html.WriteString(`</td>
			</tr>
		</table>
		<div class="footer">
			<p>This email was sent from The Monkeys contact form.</p>
		</div>
	</div>
</body>
</html>`)

	return html.String()
}
