package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"lattice-web/backend/models"
)

const (
	tokenTTL          = 24 * time.Hour
	contextUserKey    = "authUser"
	bcryptCost        = bcrypt.DefaultCost
	authUserNameLimit = 100
)

// weakSecrets are placeholder values shipped in examples; using them in
// production is unsafe so we warn loudly when one is detected.
var weakSecrets = map[string]struct{}{
	"":                                    {},
	"your-secret-key-here":                {},
	"your-jwt-secret-key-here-change-in-production": {},
	"changeme": {},
}

// authService holds the dependencies for local authentication.
type authService struct {
	db     *gorm.DB
	secret []byte
}

// newAuthService builds the auth service, resolving the signing secret from the
// environment. An empty secret falls back to an ephemeral random value so the
// server never runs with a publicly known key; tokens then reset on restart.
func newAuthService(db *gorm.DB) *authService {
	raw := os.Getenv("JWT_SECRET")
	if _, weak := weakSecrets[raw]; weak {
		if raw == "" {
			log.Println("WARNING: JWT_SECRET is not set; generating an ephemeral secret. Sessions will not survive restarts. Set JWT_SECRET for production.")
		} else {
			log.Println("WARNING: JWT_SECRET is set to a known placeholder value; replace it with a strong random secret for production.")
		}
		if raw == "" {
			buf := make([]byte, 32)
			if _, err := rand.Read(buf); err != nil {
				log.Fatalf("failed to generate ephemeral JWT secret: %v", err)
			}
			raw = hex.EncodeToString(buf)
		}
	}
	return &authService{db: db, secret: []byte(raw)}
}

type authClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (a *authService) generateToken(user models.User) (string, error) {
	claims := authClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

func (a *authService) parseToken(tokenStr string) (*authClaims, error) {
	claims := &authClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Reject any algorithm other than the one we sign with.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func hashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// middleware enforces a valid bearer token on every request it guards and loads
// the authenticated user into the request context.
func (a *authService) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization required"})
			return
		}
		tokenStr := strings.TrimSpace(header[len(prefix):])
		claims, err := a.parseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid or expired token"})
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token subject"})
			return
		}
		var user models.User
		if err := a.db.Where("id = ?", userID).First(&user).Error; err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "user no longer exists"})
			return
		}
		c.Set(contextUserKey, user)
		c.Next()
	}
}

// currentUser returns the authenticated user previously set by the middleware.
func currentUser(c *gin.Context) (models.User, bool) {
	v, ok := c.Get(contextUserKey)
	if !ok {
		return models.User{}, false
	}
	user, ok := v.(models.User)
	return user, ok
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func (a *authService) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	password, err := validatePassword(req.Password)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = email
	}
	if len(name) > authUserNameLimit {
		c.JSON(400, gin.H{"error": "name is too long"})
		return
	}

	var existing int64
	a.db.Model(&models.User{}).Where("email = ?", email).Count(&existing)
	if existing > 0 {
		c.JSON(409, gin.H{"error": "an account with this email already exists"})
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create account"})
		return
	}

	// The first registered account becomes admin; everyone else is a user.
	var total int64
	a.db.Model(&models.User{}).Count(&total)
	role := "user"
	if total == 0 {
		role = "admin"
	}

	user := models.User{
		Email:        email,
		Name:         name,
		PasswordHash: hash,
		Provider:     "local",
		ProviderID:   email,
		Role:         role,
		LastLoginAt:  time.Now(),
	}
	if err := a.db.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed to create account"})
		return
	}

	token, err := a.generateToken(user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to issue token"})
		return
	}
	c.JSON(201, authResponse{Token: token, User: user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *authService) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if req.Password == "" {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	var user models.User
	err = a.db.Where("email = ? AND provider = ?", email, "local").First(&user).Error
	if err != nil || user.PasswordHash == "" || !checkPassword(user.PasswordHash, req.Password) {
		// Uniform error + a hash comparison cost even on missing users would be
		// ideal; we keep the message uniform to avoid user enumeration.
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	a.db.Model(&user).Update("last_login_at", time.Now())

	token, err := a.generateToken(user)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to issue token"})
		return
	}
	c.JSON(200, authResponse{Token: token, User: user})
}

func (a *authService) me(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	c.JSON(200, user)
}

func (a *authService) logout(c *gin.Context) {
	// Tokens are stateless; logout is handled client-side by discarding the
	// token. Endpoint exists for symmetry and future revocation support.
	c.JSON(200, gin.H{"message": "logged out"})
}
