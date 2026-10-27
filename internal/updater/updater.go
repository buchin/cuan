package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UpdateInfo holds application version and release status.
type UpdateInfo struct {
	CurrentVersion    string     `json:"current_version"`
	CurrentCommit     string     `json:"current_commit"`
	LatestVersion     string     `json:"latest_version"`
	HasUpdate         bool       `json:"has_update"`
	ReleaseNotes      string     `json:"release_notes,omitempty"`
	ReleaseURL        string     `json:"release_url,omitempty"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	AutoUpdateEnabled bool       `json:"auto_update_enabled"`
	AutoUpdateTime    string     `json:"auto_update_time"` // default "03:00"
	LastError         string     `json:"last_error,omitempty"`
}

// SettingsRequest is the payload to update preferences.
type SettingsRequest struct {
	AutoUpdateEnabled *bool   `json:"auto_update_enabled,omitempty"`
	AutoUpdateTime    *string `json:"auto_update_time,omitempty"`
}

// Config provides initial parameters for the updater service.
type Config struct {
	CurrentVersion string
	CurrentCommit  string
	StoragePath    string // path to persist update preferences JSON
	RepoOwner      string // default "buchin"
	RepoName       string // default "cuan"
	HTTPClient     *http.Client
}

// Service manages version inspection and update preferences.
type Service struct {
	mu             sync.RWMutex
	currentVersion string
	currentCommit  string
	storagePath    string
	repoOwner      string
	repoName       string
	httpClient     *http.Client

	info UpdateInfo
}

// NewService creates a new updater service and loads persisted settings.
func NewService(cfg Config) *Service {
	owner := cfg.RepoOwner
	if owner == "" {
		owner = "buchin"
	}
	repo := cfg.RepoName
	if repo == "" {
		repo = "cuan"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	ver := cfg.CurrentVersion
	if ver == "" {
		ver = "0.1.0"
	}

	svc := &Service{
		currentVersion: ver,
		currentCommit:  cfg.CurrentCommit,
		storagePath:    cfg.StoragePath,
		repoOwner:      owner,
		repoName:       repo,
		httpClient:     client,
		info: UpdateInfo{
			CurrentVersion:    ver,
			CurrentCommit:     cfg.CurrentCommit,
			LatestVersion:     ver,
			HasUpdate:         false,
			AutoUpdateEnabled: true,
			AutoUpdateTime:    "03:00",
		},
	}

	svc.loadSettings()
	return svc
}

func (s *Service) loadSettings() {
	if s.storagePath == "" {
		return
	}
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return
	}
	var saved struct {
		AutoUpdateEnabled bool   `json:"auto_update_enabled"`
		AutoUpdateTime    string `json:"auto_update_time"`
	}
	if err := json.Unmarshal(data, &saved); err == nil {
		s.info.AutoUpdateEnabled = saved.AutoUpdateEnabled
		if saved.AutoUpdateTime != "" {
			s.info.AutoUpdateTime = saved.AutoUpdateTime
		}
	}
}

func (s *Service) saveSettings() error {
	if s.storagePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.storagePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]interface{}{
		"auto_update_enabled": s.info.AutoUpdateEnabled,
		"auto_update_time":    s.info.AutoUpdateTime,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storagePath, data, 0644)
}

// GetInfo returns the current update state.
func (s *Service) GetInfo() UpdateInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

// UpdateSettings modifies auto-update preferences and saves to disk.
func (s *Service) UpdateSettings(req SettingsRequest) (UpdateInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.AutoUpdateEnabled != nil {
		s.info.AutoUpdateEnabled = *req.AutoUpdateEnabled
	}
	if req.AutoUpdateTime != nil {
		val := strings.TrimSpace(*req.AutoUpdateTime)
		if val != "" {
			s.info.AutoUpdateTime = val
		}
	}

	_ = s.saveSettings()
	return s.info, nil
}

// githubReleaseDTO maps GitHub's latest release response.
type githubReleaseDTO struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

// CheckForUpdate queries GitHub for the latest published release.
func (s *Service) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", s.repoOwner, s.repoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		s.recordError(err.Error())
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Cuan-UpdateChecker/"+s.currentVersion)

	resp, err := s.httpClient.Do(req)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.info.LastCheckedAt = &now

	if err != nil {
		s.info.LastError = fmt.Sprintf("Koneksi gagal: %v", err)
		return &s.info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No release published yet, current version is up to date
		s.info.LatestVersion = s.currentVersion
		s.info.HasUpdate = false
		s.info.LastError = ""
		return &s.info, nil
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("GitHub response code: %d", resp.StatusCode)
		s.info.LastError = msg
		return &s.info, fmt.Errorf("%s", msg)
	}

	var rel githubReleaseDTO
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		s.info.LastError = fmt.Sprintf("Gagal parse release: %v", err)
		return &s.info, err
	}

	latestTag := strings.TrimPrefix(rel.TagName, "v")
	s.info.LatestVersion = latestTag
	s.info.ReleaseNotes = rel.Body
	s.info.ReleaseURL = rel.HTMLURL
	s.info.PublishedAt = &rel.PublishedAt
	s.info.LastError = ""

	// Check if latest version differs from current version
	if latestTag != "" && latestTag != s.currentVersion {
		s.info.HasUpdate = compareVersions(latestTag, s.currentVersion) > 0
	} else {
		s.info.HasUpdate = false
	}

	return &s.info, nil
}

func (s *Service) recordError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.info.LastCheckedAt = &now
	s.info.LastError = msg
}

// compareVersions returns 1 if v1 > v2, -1 if v1 < v2, and 0 if equal.
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")
	if v1 == v2 {
		return 0
	}
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			_, _ = fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			_, _ = fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}
