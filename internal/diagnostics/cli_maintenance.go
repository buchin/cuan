package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ScheduledJobState struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	LastStatus string     `json:"last_status"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
}

func (c *CuanClient) ScheduledJobs(ctx context.Context) ([]ScheduledJobState, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	response, err := c.doRequest(ctx, http.MethodGet, "/api/v1/system/scheduler/jobs", nil)
	if err != nil {
		return nil, errors.New("Scheduler state could not be read")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Scheduler state unavailable (HTTP %d)", response.StatusCode)
	}
	var result struct {
		Jobs []ScheduledJobState `json:"jobs"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil {
		return nil, errors.New("Invalid scheduler state")
	}
	if result.Jobs == nil {
		result.Jobs = []ScheduledJobState{}
	}
	return result.Jobs, nil
}

func RunMaintenanceCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(out, "Usage: cuan maintenance <status|run>\nRead or request the installed storage maintenance job through the configured Owner connection. Run acknowledges scheduling only; inspect status for completion.")
		return 0
	}
	if len(args) != 1 || (args[0] != "status" && args[0] != "run") {
		fmt.Fprintln(errOut, "Use cuan maintenance status or run")
		return 2
	}
	if err := taskSessionOwnerWriteError(); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Connect Cuan before inspecting maintenance")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	jobs, err := client.ScheduledJobs(ctx)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	var job *ScheduledJobState
	for i := range jobs {
		if jobs[i].ID == "storage_maintenance" {
			job = &jobs[i]
			break
		}
	}
	if job == nil {
		fmt.Fprintln(errOut, "Storage maintenance is not registered on this server")
		return 1
	}
	if args[0] == "status" {
		return encodeMaintenance(out, errOut, job)
	}
	if !job.Enabled {
		fmt.Fprintln(errOut, "Storage maintenance is unavailable on this installation")
		return 1
	}
	response, err := client.doRequest(ctx, http.MethodPost, "/api/v1/system/scheduler/jobs/storage_maintenance/trigger", nil)
	if err != nil {
		fmt.Fprintln(errOut, "Maintenance request could not be confirmed; inspect status before retrying")
		return 1
	}
	defer response.Body.Close()
	var receipt struct {
		Status string `json:"status"`
		JobID  string `json:"job_id"`
	}
	if response.StatusCode != http.StatusAccepted || json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&receipt) != nil || receipt.JobID != job.ID || receipt.Status != "triggered" {
		fmt.Fprintln(errOut, "Maintenance request could not be confirmed; inspect status before retrying")
		return 1
	}
	return encodeMaintenance(out, errOut, receipt)
}
func encodeMaintenance(out, errOut io.Writer, value any) int {
	if err := json.NewEncoder(out).Encode(value); err != nil {
		fmt.Fprintln(errOut, "Could not write maintenance result")
		return 1
	}
	return 0
}
