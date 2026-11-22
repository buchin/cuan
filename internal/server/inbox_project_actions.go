package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

type inboxProjectAction struct {
	Action      string `json:"action"` // "create", "update", "archive", "unarchive", "set_env"
	ProjectID   string `json:"project_id,omitempty"`
	Name        string `json:"name,omitempty"`
	RepoURL     string `json:"repo_url,omitempty"`
	Description string `json:"description,omitempty"`
	EnvKey      string `json:"env_key,omitempty"`
	EnvValue    string `json:"env_value,omitempty"`
}

func parseInboxProject(content string) (string, *inboxProjectAction, error) {
	const marker = "```cuan-project\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid project action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxProjectAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	if a.Action == "" {
		return "", nil, errors.New("Action is required.")
	}
	if a.Action == "create" && strings.TrimSpace(a.Name) == "" {
		return "", nil, errors.New("Project name is required.")
	}
	if (a.Action == "update" || a.Action == "archive" || a.Action == "unarchive") && strings.TrimSpace(a.ProjectID) == "" {
		return "", nil, errors.New("Project ID is required.")
	}
	if a.Action == "set_env" {
		if strings.TrimSpace(a.ProjectID) == "" {
			return "", nil, errors.New("Project ID is required.")
		}
		if strings.TrimSpace(a.EnvKey) == "" {
			return "", nil, errors.New("Environment key is required.")
		}
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func generateManagedProjectID(name string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		h := sha256.Sum256([]byte(name + fmt.Sprint(time.Now().UnixNano())))
		copy(b, h[:16])
	}
	return "proj_" + hex.EncodeToString(b)
}

func parseEnvLines(raw string) map[string]string {
	res := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if ok {
			res[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return res
}

func serializeEnvLines(m map[string]string) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return strings.Join(lines, "\n")
}

func (s *Server) applyInboxProject(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxProject(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.projectSvc == nil {
		return cleanText + "\n\n⚠️ Project service is unavailable: service is not configured."
	}

	switch action.Action {
	case "create":
		name := strings.TrimSpace(action.Name)
		projID := strings.TrimSpace(action.ProjectID)
		if projID == "" || !strings.HasPrefix(projID, "proj_") || len(projID) != 37 {
			projID = generateManagedProjectID(name)
		}
		proj, err := s.projectSvc.CreateManagedWorkspace(ctx, projID, name)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal membuat project: %v", err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Project Baru Berhasil Dibuat:** `%s` (`%s`)", proj.Name, proj.ID)

	case "update":
		curr, err := s.projectSvc.Get(ctx, action.ProjectID)
		if err != nil || curr == nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Project tidak ditemukan: %s", action.ProjectID)
		}
		settings := project.SettingsFromMetadata(curr.Metadata)
		name := curr.Name
		if strings.TrimSpace(action.Name) != "" {
			name = strings.TrimSpace(action.Name)
		}
		goal := settings.Goal
		if strings.TrimSpace(action.Description) != "" {
			goal = strings.TrimSpace(action.Description)
		}
		repo := curr.RepoURL
		if strings.TrimSpace(action.RepoURL) != "" {
			repo = strings.TrimSpace(action.RepoURL)
		}
		req := project.UpdateSettingsRequest{
			Name:              name,
			Goal:              goal,
			Instructions:      settings.Instructions,
			RepoURL:           repo,
			DefaultBranch:     curr.DefaultBranch,
			DeployBranch:      settings.DeployBranch,
			DeployScript:      settings.DeployScript,
			AutoDeploy:        settings.AutoDeploy,
			Environment:       settings.Environment,
			ExpectedUpdatedAt: curr.UpdatedAt,
		}
		updated, err := s.projectSvc.UpdateSettings(ctx, action.ProjectID, req)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memperbarui project %s: %v", action.ProjectID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Project Diperbarui:** `%s` (`%s`)", updated.Name, updated.ID)

	case "archive":
		proj, err := s.projectSvc.Archive(ctx, action.ProjectID)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal mengarsipkan project %s: %v", action.ProjectID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Project Diarsipkan:** `%s` (`%s`)", proj.Name, proj.ID)

	case "unarchive":
		proj, err := s.projectSvc.Unarchive(ctx, action.ProjectID)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal mengaktifkan kembali project %s: %v", action.ProjectID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Project Diaktifkan Kembali:** `%s` (`%s`)", proj.Name, proj.ID)

	case "set_env":
		curr, err := s.projectSvc.Get(ctx, action.ProjectID)
		if err != nil || curr == nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Project tidak ditemukan: %s", action.ProjectID)
		}
		settings := project.SettingsFromMetadata(curr.Metadata)
		envMap := parseEnvLines(settings.Environment)
		envMap[strings.TrimSpace(action.EnvKey)] = strings.TrimSpace(action.EnvValue)
		settings.Environment = serializeEnvLines(envMap)

		req := project.UpdateSettingsRequest{
			Name:              curr.Name,
			Goal:              settings.Goal,
			Instructions:      settings.Instructions,
			RepoURL:           curr.RepoURL,
			DefaultBranch:     curr.DefaultBranch,
			DeployBranch:      settings.DeployBranch,
			DeployScript:      settings.DeployScript,
			AutoDeploy:        settings.AutoDeploy,
			Environment:       settings.Environment,
			ExpectedUpdatedAt: curr.UpdatedAt,
		}
		if _, err := s.projectSvc.UpdateSettings(ctx, action.ProjectID, req); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memperbarui env project: %v", err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Environment Variable Diperbarui:** `%s` di project `%s`.", action.EnvKey, curr.Name)

	default:
		return cleanText
	}
}
