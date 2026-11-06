package task

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

var failedGoPackage = regexp.MustCompile(`(?m)^FAIL[\t ]+[^\s]+/(internal|pkg)/([A-Za-z0-9_./-]+)[\t ]+`)
var failedGoFile = regexp.MustCompile(`(?m)^[\t ]+([A-Za-z0-9_.-]+_test\.go):[0-9]+:`)

// FailedCheckRevalidationPaths identifies an actual assertion failure in an
// native inspection with an authenticated source-boundary observation. It does not turn that failure into a pass. A
// later revalidation still requires scoped review, changed named files, the
// existing single allowance and fresh verification on the new live revision.
func FailedCheckRevalidationPaths(d *TaskDetailResponse) []string {
	if d == nil || d.LatestAttempt == nil {
		return nil
	}
	a := d.LatestAttempt
	if a.TaskID != d.ID || a.Status != AttemptStatusFailed || a.ErrorMessage != "Live result inspection project checks did not pass" || a.ExitCode == nil || *a.ExitCode != 0 || a.RunnerID == "" || a.FinishedAt == nil || a.CancelRequested || a.CanceledAt != nil || !liveImplementationFinalReport(d.Activities, d.ID, a.ID) {
		return nil
	}
	summary := ""
	unchanged := false
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != a.ID || event.Author != a.RunnerID {
			continue
		}
		if event.ID == "verification:"+a.ID && event.ActivityType == ActivityTypeRunnerVerification {
			var record RunnerVerificationRecord
			if json.Unmarshal([]byte(event.Message), &record) == nil {
				summary = record.Summary
			}
		}
		if event.ID == "repository-observation:"+a.ID && event.ActivityType == "repository_scope_observed" && event.NewStatus == AttemptStatusFailed {
			var record struct {
				Baseline string `json:"baseline"`
				Final    string `json:"final"`
			}
			if json.Unmarshal([]byte(event.Message), &record) == nil && validVerificationHash(record.Baseline) && record.Baseline == record.Final {
				unchanged = true
			}
		}
	}
	first := strings.SplitN(summary, "\n", 2)[0]
	if !unchanged || (first != "Project verification: test.sh --fast (exit 1)" && first != "Project verification: go test ./... (exit 1)") || !strings.Contains(summary, "--- FAIL: ") {
		return nil
	}
	packages := failedGoPackage.FindAllStringSubmatch(summary, -1)
	if len(packages) != 1 {
		return nil
	}
	directory := packages[0][1] + "/" + packages[0][2]
	if path.Clean(directory) != directory || strings.Contains(directory, "../") {
		return nil
	}
	unique := map[string]bool{}
	for _, file := range failedGoFile.FindAllStringSubmatch(summary, -1) {
		unique[directory+"/"+file[1]] = true
	}
	if len(unique) == 0 || len(unique) > 16 {
		return nil
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
