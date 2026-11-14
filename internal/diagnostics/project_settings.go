package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProjectSettingsResult struct {
	ProjectID     string `json:"project_id"`
	Name          string `json:"name"`
	RepoURL       string `json:"repo_url"`
	DefaultBranch string `json:"default_branch"`
	DeployBranch  string `json:"deploy_branch"`
	DeployScript  string `json:"deploy_script"`
	AutoDeploy    bool   `json:"auto_deploy"`
	Goal          string `json:"goal"`
	Instructions  string `json:"instructions"`
	UpdatedAt     any    `json:"updated_at"`
}

func settingsResult(p map[string]any) ProjectSettingsResult {
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	id, _ := p["id"].(string)
	name, _ := p["name"].(string)
	repoURL, _ := p["repo_url"].(string)
	defaultBranch, _ := p["default_branch"].(string)
	deployBranch, _ := settings["deploy_branch"].(string)
	deployScript, _ := settings["deploy_script"].(string)
	autoDeploy, _ := settings["auto_deploy"].(bool)
	goal, _ := settings["goal"].(string)
	instructions, _ := settings["instructions"].(string)
	return ProjectSettingsResult{
		ProjectID:     id,
		Name:          name,
		RepoURL:       repoURL,
		DefaultBranch: defaultBranch,
		DeployBranch:  deployBranch,
		DeployScript:  deployScript,
		AutoDeploy:    autoDeploy,
		Goal:          goal,
		Instructions:  instructions,
		UpdatedAt:     p["updated_at"],
	}
}

func (c *CuanClient) GetProjectSettings(ctx context.Context, id string) (ProjectSettingsResult, error) {
	p, err := c.environmentProject(ctx, id)
	if err != nil {
		return ProjectSettingsResult{}, err
	}
	return settingsResult(p), nil
}

func (c *CuanClient) UpdateProjectSettings(ctx context.Context, id string, deployScript, deployBranch *string, autoDeploy *bool) (ProjectSettingsResult, error) {
	if deployScript == nil && deployBranch == nil && autoDeploy == nil {
		return ProjectSettingsResult{}, errors.New("provide at least one setting to update")
	}
	if deployScript != nil && len([]rune(*deployScript)) > 10000 {
		return ProjectSettingsResult{}, errors.New("deploy script exceeds allowed length (10,000 characters)")
	}
	p, err := c.environmentProject(ctx, id)
	if err != nil {
		return ProjectSettingsResult{}, err
	}
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	payload := map[string]any{"expected_updated_at": p["updated_at"]}
	for _, key := range []string{"name", "repo_url", "default_branch"} {
		payload[key] = p[key]
	}
	for _, key := range []string{"goal", "instructions", "deploy_branch", "deploy_script", "environment"} {
		value, _ := settings[key].(string)
		payload[key] = value
	}
	if value, ok := settings["auto_deploy"].(bool); ok {
		payload["auto_deploy"] = value
	}
	if deployScript != nil {
		payload["deploy_script"] = strings.TrimSpace(*deployScript)
	}
	if deployBranch != nil {
		payload["deploy_branch"] = strings.TrimSpace(*deployBranch)
	}
	if autoDeploy != nil {
		payload["auto_deploy"] = *autoDeploy
	}
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(id)+"/settings", payload)
	if err != nil {
		return ProjectSettingsResult{}, errors.New("could not save project settings")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ProjectSettingsResult{}, errors.New("project settings changed concurrently; reload and retry")
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ProjectSettingsResult{}, fmt.Errorf("could not update project settings (%d): %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var updated map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256000)).Decode(&updated); err != nil {
		return ProjectSettingsResult{}, err
	}
	return settingsResult(updated), nil
}

func runProjectSettingsCLI(args []string, out, errOut io.Writer) int {
	const help = "Usage: cuan project settings show PROJECT_ID | set PROJECT_ID [--deploy-script TEXT] [--auto-deploy true|false] [--deploy-branch TEXT]\nOnly explicitly supplied settings change. Environment and other fields stay intact."
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, help)
		return 0
	}
	action := args[0]
	if action != "show" && action != "set" {
		fmt.Fprintln(errOut, help)
		return 2
	}
	if len(args) < 2 || strings.HasPrefix(args[1], "-") {
		fmt.Fprintln(errOut, "project ID is required")
		return 2
	}
	id := args[1]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := GetCuanClient()

	if action == "show" {
		res, err := client.GetProjectSettings(ctx, id)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := json.NewEncoder(out).Encode(res); err != nil {
			fmt.Fprintln(errOut, "Could not encode project settings.")
			return 1
		}
		return 0
	}

	fs := flag.NewFlagSet("project settings set", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var deployScript, deployBranch, autoDeployStr string
	fs.StringVar(&deployScript, "deploy-script", "", "Deploy script to execute on release")
	fs.StringVar(&deployBranch, "deploy-branch", "", "Target branch for deployment")
	fs.StringVar(&autoDeployStr, "auto-deploy", "", "Enable or disable auto-deploy (true/false, on/off, 1/0)")

	if err := fs.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	var dsPtr, dbPtr *string
	var adPtr *bool

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "deploy-script":
			dsPtr = &deployScript
		case "deploy-branch":
			dbPtr = &deployBranch
		case "auto-deploy":
			v := strings.ToLower(strings.TrimSpace(autoDeployStr))
			val := v == "true" || v == "on" || v == "1" || v == "yes"
			adPtr = &val
		}
	})

	if dsPtr == nil && dbPtr == nil && adPtr == nil {
		fmt.Fprintln(errOut, "provide at least one setting to update: --deploy-script, --deploy-branch, or --auto-deploy")
		return 2
	}

	res, err := client.UpdateProjectSettings(ctx, id, dsPtr, dbPtr, adPtr)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(res); err != nil {
		fmt.Fprintln(errOut, "Could not encode updated project settings.")
		return 1
	}
	return 0
}
