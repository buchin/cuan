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

type ProjectGuidanceResult struct {
	ProjectID    string `json:"project_id"`
	Goal         string `json:"goal"`
	Instructions string `json:"instructions"`
	UpdatedAt    any    `json:"updated_at"`
}

func guidanceResult(p map[string]any) ProjectGuidanceResult {
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	id, _ := p["id"].(string)
	goal, _ := settings["goal"].(string)
	instructions, _ := settings["instructions"].(string)
	return ProjectGuidanceResult{id, goal, instructions, p["updated_at"]}
}
func (c *CuanClient) UpdateProjectGuidance(ctx context.Context, id string, goal, instructions *string) (ProjectGuidanceResult, error) {
	if goal == nil && instructions == nil {
		return ProjectGuidanceResult{}, errors.New("provide --goal or --instructions")
	}
	if goal != nil && len([]rune(*goal)) > 2000 || instructions != nil && len([]rune(*instructions)) > 4000 {
		return ProjectGuidanceResult{}, errors.New("goal or instructions exceed the allowed length")
	}
	p, err := c.environmentProject(ctx, id)
	if err != nil {
		return ProjectGuidanceResult{}, err
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
	if goal != nil {
		payload["goal"] = strings.TrimSpace(*goal)
	}
	if instructions != nil {
		payload["instructions"] = strings.TrimSpace(*instructions)
	}
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(id)+"/settings", payload)
	if err != nil {
		return ProjectGuidanceResult{}, errors.New("could not save project guidance")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ProjectGuidanceResult{}, errors.New("project changed elsewhere; review current settings and retry")
	}
	if resp.StatusCode != http.StatusOK {
		return ProjectGuidanceResult{}, fmt.Errorf("could not save project guidance (status %d)", resp.StatusCode)
	}
	var saved map[string]any
	if json.NewDecoder(io.LimitReader(resp.Body, 256000)).Decode(&saved) != nil {
		return ProjectGuidanceResult{}, errors.New("could not read saved project guidance")
	}
	return guidanceResult(saved), nil
}
func runProjectGuidanceCLI(args []string, out, errOut io.Writer) int {
	const help = "Usage: cuan project guidance show PROJECT_ID | set PROJECT_ID [--goal TEXT] [--instructions TEXT]\nOnly explicitly supplied guidance fields change. Other settings and environment stay intact. New tasks capture a snapshot; existing tasks retain theirs."
	if len(args) == 0 || args[0] == "--help" {
		fmt.Fprintln(out, help)
		return 0
	}
	if len(args) < 2 || (args[0] != "show" && args[0] != "set") {
		fmt.Fprintln(errOut, help)
		return 2
	}
	flags := flag.NewFlagSet("project guidance", flag.ContinueOnError)
	flags.SetOutput(errOut)
	var goal, instructions string
	if args[0] == "set" {
		flags.StringVar(&goal, "goal", "", "Project outcome (up to 2000 characters)")
		flags.StringVar(&instructions, "instructions", "", "Standing project instructions (up to 4000 characters)")
	}
	if err := flags.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, help)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := GetCuanClient()
	var result ProjectGuidanceResult
	var err error
	if args[0] == "show" {
		var p map[string]any
		p, err = client.environmentProject(ctx, args[1])
		if err == nil {
			result = guidanceResult(p)
		}
	} else {
		var g, i *string
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "goal" {
				g = &goal
			}
			if f.Name == "instructions" {
				i = &instructions
			}
		})
		result, err = client.UpdateProjectGuidance(ctx, args[1], g, i)
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return 1
	}
	return 0
}
