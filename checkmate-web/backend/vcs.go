package main

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"checkmate-web/backend/models"
)

// RemoteRepo is a provider-neutral repository listing entry.
type RemoteRepo struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url,omitempty"`
}

type vcsService struct {
	db   *gorm.DB
	box  *secretBox
	http *http.Client
}

func newVcsService(db *gorm.DB, box *secretBox) *vcsService {
	return &vcsService{
		db:   db,
		box:  box,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func githubAppConfigured() bool {
	if os.Getenv("GITHUB_APP_MOCK") == "1" || strings.EqualFold(os.Getenv("GITHUB_APP_MOCK"), "true") {
		return true
	}
	return strings.TrimSpace(os.Getenv("GITHUB_APP_ID")) != "" &&
		(strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY")) != "" ||
			strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")) != "")
}

func (v *vcsService) providers(c *gin.Context) {
	c.JSON(200, gin.H{
		"github_app": gin.H{
			"configured": githubAppConfigured(),
			"mock":       os.Getenv("GITHUB_APP_MOCK") == "1" || strings.EqualFold(os.Getenv("GITHUB_APP_MOCK"), "true"),
			"slug":       os.Getenv("GITHUB_APP_SLUG"),
		},
		"generic_pat": gin.H{"configured": true},
		"gitlab_pat":  gin.H{"configured": true},
		"bitbucket_pat": gin.H{"configured": true},
	})
}

func (v *vcsService) listConnections(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	var conns []models.VcsConnection
	if err := v.db.Where("created_by = ?", user.ID).Order("created_at DESC").Find(&conns).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, conns)
}

type createConnectionRequest struct {
	Provider    string `json:"provider"` // github | gitlab | bitbucket | generic
	Host        string `json:"host"`
	DisplayName string `json:"display_name"`
	Token       string `json:"token"`
	CloneUser   string `json:"clone_user"`
	ExternalID  string `json:"external_id"`
}

func (v *vcsService) createConnection(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	var req createConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	switch provider {
	case "github", "gitlab", "bitbucket", "generic":
	default:
		c.JSON(400, gin.H{"error": "provider must be github, gitlab, bitbucket, or generic"})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(400, gin.H{"error": "token is required"})
		return
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		switch provider {
		case "github":
			host = "github.com"
		case "gitlab":
			host = "gitlab.com"
		case "bitbucket":
			host = "bitbucket.org"
		default:
			c.JSON(400, gin.H{"error": "host is required for generic provider"})
			return
		}
	}
	cloneUser := strings.TrimSpace(req.CloneUser)
	if cloneUser == "" {
		switch provider {
		case "github":
			cloneUser = "x-access-token"
		case "gitlab":
			cloneUser = "oauth2"
		case "bitbucket":
			cloneUser = "x-token-auth"
		default:
			cloneUser = "git"
		}
	}
	enc, err := v.box.Encrypt(token)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to store secret"})
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		name = provider + "@" + host
	}
	conn := models.VcsConnection{
		Provider:    provider,
		AuthType:    "pat",
		Host:        host,
		ExternalID:  strings.TrimSpace(req.ExternalID),
		DisplayName: name,
		SecretEnc:   enc,
		CloneUser:   cloneUser,
		CreatedBy:   user.ID,
	}
	if err := v.db.Create(&conn).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, conn)
}

func (v *vcsService) deleteConnection(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	id, err := validateUUID(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	res := v.db.Where("id = ? AND created_by = ?", id, user.ID).Delete(&models.VcsConnection{})
	if res.Error != nil {
		c.JSON(500, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "connection not found"})
		return
	}
	c.JSON(204, nil)
}

func (v *vcsService) listRepos(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	id, err := validateUUID(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var conn models.VcsConnection
	if err := v.db.Where("id = ? AND created_by = ?", id, user.ID).First(&conn).Error; err != nil {
		c.JSON(404, gin.H{"error": "connection not found"})
		return
	}
	repos, err := v.fetchRepos(&conn)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"repos": repos})
}

func (v *vcsService) fetchRepos(conn *models.VcsConnection) ([]RemoteRepo, error) {
	if conn.AuthType == "github_app" || (conn.Provider == "github" && conn.AuthType == "github_app") {
		return v.githubListRepos(conn)
	}
	// PAT-based: GitHub user repos, GitLab projects, Bitbucket — best-effort; generic returns empty list.
	token, err := v.box.Decrypt(conn.SecretEnc)
	if err != nil {
		return nil, err
	}
	switch conn.Provider {
	case "github":
		return v.githubPATListRepos(token)
	case "gitlab":
		return v.gitlabListProjects(conn.Host, token)
	case "bitbucket":
		return v.bitbucketListRepos(conn.Host, token)
	default:
		return []RemoteRepo{}, nil
	}
}

// MintCloneCreds returns HTTPS clone username/password for a project connection.
func (v *vcsService) MintCloneCreds(conn *models.VcsConnection) (user, pass string, err error) {
	if conn == nil {
		return "", "", nil
	}
	if conn.AuthType == "github_app" {
		if mockGitHubApp() {
			return "x-access-token", "mock-token", nil
		}
		instID, err := strconv.ParseInt(conn.ExternalID, 10, 64)
		if err != nil {
			return "", "", fmt.Errorf("invalid installation id")
		}
		tok, err := v.githubInstallationToken(instID)
		if err != nil {
			return "", "", err
		}
		return "x-access-token", tok, nil
	}
	pass, err = v.box.Decrypt(conn.SecretEnc)
	if err != nil {
		return "", "", err
	}
	user = conn.CloneUser
	if user == "" {
		user = "git"
	}
	return user, pass, nil
}

func mockGitHubApp() bool {
	return os.Getenv("GITHUB_APP_MOCK") == "1" || strings.EqualFold(os.Getenv("GITHUB_APP_MOCK"), "true")
}

func (v *vcsService) githubInstallURL(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	if mockGitHubApp() {
		// Simulated install: create connection immediately and return mock redirect.
		conn := models.VcsConnection{
			Provider:    "github",
			AuthType:    "github_app",
			Host:        "github.com",
			ExternalID:  "1",
			DisplayName: "mock-org (GitHub App)",
			CreatedBy:   user.ID,
		}
		// Upsert by external id + user for mock.
		var existing models.VcsConnection
		if err := v.db.Where("created_by = ? AND provider = ? AND external_id = ?", user.ID, "github", "1").First(&existing).Error; err == nil {
			c.JSON(200, gin.H{
				"url":             uiURL() + "/integrations/github?installed=1",
				"mock":            true,
				"connection_id":   existing.ID,
				"installation_id": "1",
			})
			return
		}
		if err := v.db.Create(&conn).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"url":             uiURL() + "/integrations/github?installed=1",
			"mock":            true,
			"connection_id":   conn.ID,
			"installation_id": "1",
		})
		return
	}
	if !githubAppConfigured() {
		c.JSON(503, gin.H{"error": "GitHub App is not configured (set GITHUB_APP_ID and private key, or GITHUB_APP_MOCK=1)"})
		return
	}
	slug := strings.TrimSpace(os.Getenv("GITHUB_APP_SLUG"))
	if slug == "" {
		c.JSON(503, gin.H{"error": "GITHUB_APP_SLUG is required for install URL"})
		return
	}
	state := user.ID.String() + ":" + uuid.NewString()
	// state is not persisted server-side for simplicity; callback trusts installation_id + authenticated user.
	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s", url.PathEscape(slug), url.QueryEscape(state))
	c.JSON(200, gin.H{"url": installURL, "mock": false})
}

func (v *vcsService) githubCallback(c *gin.Context) {
	installationID := c.Query("installation_id")
	if installationID == "" {
		c.JSON(400, gin.H{"error": "installation_id required"})
		return
	}
	// Prefer Authorization if SPA calls this; also support redirect with setup after user is logged in via cookie is not available — UI should call with JWT after redirect.
	user, ok := currentUser(c)
	if !ok {
		// Redirect to UI with query params so the SPA can finalize with JWT.
		c.Redirect(302, uiURL()+"/integrations/github?installation_id="+url.QueryEscape(installationID))
		return
	}
	conn, err := v.persistGitHubInstallation(user.ID, installationID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"connection": conn})
}

type finalizeInstallRequest struct {
	InstallationID string `json:"installation_id"`
}

func (v *vcsService) githubFinalize(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authorization required"})
		return
	}
	var req finalizeInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.InstallationID) == "" {
		c.JSON(400, gin.H{"error": "installation_id required"})
		return
	}
	conn, err := v.persistGitHubInstallation(user.ID, strings.TrimSpace(req.InstallationID))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, conn)
}

func (v *vcsService) persistGitHubInstallation(userID uuid.UUID, installationID string) (models.VcsConnection, error) {
	var existing models.VcsConnection
	if err := v.db.Where("created_by = ? AND provider = ? AND external_id = ?", userID, "github", installationID).First(&existing).Error; err == nil {
		return existing, nil
	}
	display := "GitHub App #" + installationID
	if mockGitHubApp() {
		display = "mock-org (GitHub App)"
	} else if account, err := v.githubInstallationAccount(installationID); err == nil && account != "" {
		display = account
	}
	conn := models.VcsConnection{
		Provider:    "github",
		AuthType:    "github_app",
		Host:        "github.com",
		ExternalID:  installationID,
		DisplayName: display,
		CloneUser:   "x-access-token",
		CreatedBy:   userID,
	}
	if err := v.db.Create(&conn).Error; err != nil {
		return models.VcsConnection{}, err
	}
	return conn, nil
}

func uiURL() string {
	if u := strings.TrimSpace(os.Getenv("CHECKMATE_UI_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:3000"
}

func publicAPIURL() string {
	if u := strings.TrimSpace(os.Getenv("CHECKMATE_PUBLIC_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:8080"
}

// --- GitHub App crypto / API ---

func loadGitHubPrivateKey() (*rsa.PrivateKey, error) {
	pemData := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY"))
	if pemData == "" {
		path := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"))
		if path == "" {
			return nil, errors.New("GITHUB_APP_PRIVATE_KEY not set")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		pemData = string(b)
	}
	// Support escaped newlines in env vars.
	pemData = strings.ReplaceAll(pemData, `\n`, "\n")
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to decode PEM private key")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(pemData))
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (v *vcsService) githubAppJWT() (string, error) {
	appID := strings.TrimSpace(os.Getenv("GITHUB_APP_ID"))
	if appID == "" {
		return "", errors.New("GITHUB_APP_ID not set")
	}
	key, err := loadGitHubPrivateKey()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": appID,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(key)
}

func (v *vcsService) githubInstallationToken(installationID int64) (string, error) {
	appJWT, err := v.githubAppJWT()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := v.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github installation token: %s", strings.TrimSpace(string(body)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("empty installation token")
	}
	return out.Token, nil
}

func (v *vcsService) githubInstallationAccount(installationID string) (string, error) {
	appJWT, err := v.githubAppJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/app/installations/"+installationID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := v.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github installation: %s", string(body))
	}
	var out struct {
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Account.Login, nil
}

func (v *vcsService) githubListRepos(conn *models.VcsConnection) ([]RemoteRepo, error) {
	if mockGitHubApp() {
		return []RemoteRepo{
			{
				ID:            "1",
				FullName:      "checkmate-demo/vulnerable-sample",
				CloneURL:      "https://github.com/aquasecurity/trivy.git",
				DefaultBranch: "main",
				Private:       false,
				HTMLURL:       "https://github.com/aquasecurity/trivy",
			},
			{
				ID:            "2",
				FullName:      "checkmate-demo/bandit-examples",
				CloneURL:      "https://github.com/PyCQA/bandit.git",
				DefaultBranch: "main",
				Private:       false,
				HTMLURL:       "https://github.com/PyCQA/bandit",
			},
		}, nil
	}
	instID, err := strconv.ParseInt(conn.ExternalID, 10, 64)
	if err != nil {
		return nil, err
	}
	token, err := v.githubInstallationToken(instID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list repos: %s", string(body))
	}
	var payload struct {
		Repositories []struct {
			ID            int64  `json:"id"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			Private       bool   `json:"private"`
			DefaultBranch string `json:"default_branch"`
			HTMLURL       string `json:"html_url"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]RemoteRepo, 0, len(payload.Repositories))
	for _, r := range payload.Repositories {
		out = append(out, RemoteRepo{
			ID:            strconv.FormatInt(r.ID, 10),
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			HTMLURL:       r.HTMLURL,
		})
	}
	return out, nil
}

func (v *vcsService) githubPATListRepos(token string) ([]RemoteRepo, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repos: %s", string(body))
	}
	var repos []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
		HTMLURL       string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, err
	}
	out := make([]RemoteRepo, 0, len(repos))
	for _, r := range repos {
		out = append(out, RemoteRepo{
			ID:            strconv.FormatInt(r.ID, 10),
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			HTMLURL:       r.HTMLURL,
		})
	}
	return out, nil
}

func (v *vcsService) gitlabListProjects(host, token string) ([]RemoteRepo, error) {
	api := fmt.Sprintf("https://%s/api/v4/projects?membership=true&per_page=50&order_by=last_activity_at", host)
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab projects: %s", string(body))
	}
	var projects []struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
		Visibility        string `json:"visibility"`
		WebURL            string `json:"web_url"`
	}
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, err
	}
	out := make([]RemoteRepo, 0, len(projects))
	for _, p := range projects {
		out = append(out, RemoteRepo{
			ID:            strconv.Itoa(p.ID),
			FullName:      p.PathWithNamespace,
			CloneURL:      p.HTTPURLToRepo,
			DefaultBranch: p.DefaultBranch,
			Private:       p.Visibility != "public",
			HTMLURL:       p.WebURL,
		})
	}
	return out, nil
}

func (v *vcsService) bitbucketListRepos(host, token string) ([]RemoteRepo, error) {
	// Works with repository access tokens / app passwords when Authorization is Bearer or Basic.
	api := fmt.Sprintf("https://api.%s/2.0/repositories?role=member&pagelen=50", strings.TrimPrefix(host, "www."))
	if host != "bitbucket.org" {
		api = fmt.Sprintf("https://%s/rest/api/1.0/repos?limit=50", host)
	}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bitbucket repos: %s", string(body))
	}
	// Cloud shape
	var cloud struct {
		Values []struct {
			UUID     string `json:"uuid"`
			FullName string `json:"full_name"`
			IsPrivate bool  `json:"is_private"`
			Links    struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
				Clone []struct {
					Name string `json:"name"`
					Href string `json:"href"`
				} `json:"clone"`
			} `json:"links"`
			Mainbranch struct {
				Name string `json:"name"`
			} `json:"mainbranch"`
		} `json:"values"`
	}
	if err := json.Unmarshal(body, &cloud); err == nil && len(cloud.Values) > 0 {
		out := make([]RemoteRepo, 0, len(cloud.Values))
		for _, r := range cloud.Values {
			clone := ""
			for _, c := range r.Links.Clone {
				if c.Name == "https" {
					clone = c.Href
					break
				}
			}
			out = append(out, RemoteRepo{
				ID:            r.UUID,
				FullName:      r.FullName,
				CloneURL:      clone,
				DefaultBranch: r.Mainbranch.Name,
				Private:       r.IsPrivate,
				HTMLURL:       r.Links.HTML.Href,
			})
		}
		return out, nil
	}
	return []RemoteRepo{}, nil
}

// githubWebhook handles installation lifecycle events (optional).
func (v *vcsService) githubWebhook(c *gin.Context) {
	// Signature verification can be added when GITHUB_APP_WEBHOOK_SECRET is set.
	c.JSON(200, gin.H{"ok": true})
}
