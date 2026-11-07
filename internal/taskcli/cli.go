// Package taskcli implements the task-bound agent tool built into cuan-server.
package taskcli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Capability struct {
	API          string `json:"api"`
	Token        string `json:"token"`
	TaskID       string `json:"task_id"`
	AttemptID    string `json:"attempt_id"`
	ArtifactsDir string `json:"artifacts_dir,omitempty"`
}

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && args[0] == "stop-hook" {
		return runStopHook(os.Stdin, out)
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, "Usage: cuan-server task get | report-template | push | report [--title TEXT] [--priority low|normal|high|urgent] [--summary TEXT] [--complete-note TEXT] [--progress TEXT] [--state progress|review|blocked|done] [--result-kind answer|implementation] [--report-file FILE] [--dod-item TEXT --dod-evidence TEXT ...] [--artifact RELATIVE_PATH ...]\nUses CUAN_TASK_CAPABILITY_FILE or CUAN_TASK_CAPABILITY_JSON; reports affect only the current task/session. Human instructions cannot be changed. Done requests verified implementation completion; it does not bypass independent review. Review requires summary and verification evidence; workflow changes after process exit.")
		fmt.Fprintln(out, `Optional --report-file JSON example:
{"state":"review","summary":"Actual result","complete_note":"Verification and limitations","dod_checklist":[{"text":"Verified requirement","completed":true,"evidence":"Observed output or check result"}],"commit_sha":"optional commit","test_summary":"Actual checks"}
Use task report-template for current criterion IDs/text/order, or task get when more task context is needed. If the task has saved criteria, review/done require every original criterion with unchanged ID, text and order and current observed evidence. Use completed:false with a factual gap for unmet criteria, or state blocked for a blocker. Correct rejected reports within the same attempt. Only tasks without a saved checklist may use --state review --summary TEXT --complete-note TEXT alone. Final results require an explicit state; progress checkpoints do not finish the session.`)
		fmt.Fprintln(out, "For read-only verification, write output under CUAN_TASK_ARTIFACTS_DIR and use --artifact session:PATH. Ordinary artifact paths remain relative to WORKSPACE_DIR. Both sources share the eight-file and 25 MB limits.")
		fmt.Fprintln(out, "Use task report-template to print a review JSON template with current original criteria and empty evidence. Fill actual findings before task report --report-file FILE; generating a template does not submit or accept work.")
		return 0
	}
	command := args[0]
	if command != "get" && command != "report" && command != "push" && command != "report-template" {
		fmt.Fprintln(errOut, "Unknown task command; use task --help")
		return 2
	}
	fs := flag.NewFlagSet("task "+command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	if command == "report-template" {
		fs.Usage = func() {
			fmt.Fprintln(errOut, "Usage: task report-template\nPrint review JSON with current original criteria and empty evidence. Fill actual findings before submitting; this command performs one scoped read and no report mutation.")
		}
	}
	title := fs.String("title", "", "Refined task title")
	priority := fs.String("priority", "", "Task priority")
	summary := fs.String("summary", "", "Work plan or result summary")
	note := fs.String("complete-note", "", "Actual verification evidence or blocker")
	progress := fs.String("progress", "", "Progress checkpoint")
	state := fs.String("state", "", "progress, review, blocked or done")
	kind := fs.String("result-kind", "", "answer for conversation without file changes; implementation otherwise")
	file := fs.String("report-file", "", "Structured JSON report file (never human instructions)")
	var artifacts []string
	fs.Func("artifact", "Output file relative to WORKSPACE_DIR, or session:PATH under CUAN_TASK_ARTIFACTS_DIR, to deliver privately in Inbox (repeatable)", func(path string) error { artifacts = append(artifacts, path); return nil })
	var dodItems, dodEvidence []string
	fs.Func("dod-item", "Original acceptance criterion text, unchanged (repeatable; pair with --dod-evidence)", func(item string) error { dodItems = append(dodItems, item); return nil })
	fs.Func("dod-evidence", "Observed result for the corresponding --dod-item (repeatable)", func(value string) error { dodEvidence = append(dodEvidence, value); return nil })
	if e := fs.Parse(args[1:]); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(errOut, "Unexpected arguments; use task --help")
		return 2
	}
	if command == "report-template" && fs.NFlag() != 0 {
		fmt.Fprintln(errOut, "task report-template does not accept report flags; fill the printed JSON with actual findings")
		return 2
	}
	if command != "report" && len(artifacts) > 0 {
		fmt.Fprintln(errOut, "Only task report accepts artifacts")
		return 2
	}
	var raw []byte
	if inline := os.Getenv("CUAN_TASK_CAPABILITY_JSON"); inline != "" {
		raw = []byte(inline)
	} else {
		path := os.Getenv("CUAN_TASK_CAPABILITY_FILE")
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			fmt.Fprintln(errOut, "Task capability is unavailable or not private. Run this tool inside the bound agent session.")
			return 1
		}
		raw, e = os.ReadFile(path)
		if e != nil {
			fmt.Fprintln(errOut, "Cannot read task capability")
			return 1
		}
	}
	var c Capability
	if e := json.Unmarshal(raw, &c); e != nil || c.Token == "" || c.TaskID == "" {
		fmt.Fprintln(errOut, "Invalid task capability")
		return 1
	}
	u, e := url.Parse(c.API)
	if e != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		fmt.Fprintln(errOut, "Task tool requires a loopback API")
		return 1
	}
	method := "GET"
	var body io.Reader
	if command == "report" {
		method = "POST"
		data := map[string]any{}
		if *file != "" {
			f, e := os.Open(*file)
			if e != nil {
				fmt.Fprintln(errOut, "Cannot read report file")
				return 1
			}
			decoder := json.NewDecoder(io.LimitReader(f, 64001))
			e = decoder.Decode(&data)
			f.Close()
			if e != nil {
				fmt.Fprintln(errOut, "Invalid report JSON")
				return 2
			}
		}
		if data == nil {
			data = map[string]any{}
		}
		values := map[string]*string{"title": title, "priority": priority, "summary": summary, "complete-note": note, "progress": progress, "state": state, "result-kind": kind}
		fs.Visit(func(f *flag.Flag) {
			if v, ok := values[f.Name]; ok {
				data[strings.ReplaceAll(f.Name, "-", "_")] = *v
			}
		})
		if len(dodItems) != len(dodEvidence) {
			fmt.Fprintln(errOut, "Pair each --dod-item with one --dod-evidence, or use --report-file for criteria with IDs")
			return 2
		}
		if len(dodItems) > 0 {
			if data["complete_note"] == nil || data["complete_note"] == "" {
				var sb strings.Builder
				sb.WriteString("### Bukti Verifikasi Definition of Done (DoD)\n")
				for i, itm := range dodItems {
					sb.WriteString("- [x] ")
					sb.WriteString(itm + ": " + dodEvidence[i])
					sb.WriteString("\n")
				}
				data["complete_note"] = sb.String()
			}
			if _, exists := data["dod_checklist"]; !exists {
				var items []map[string]any
				for i, itm := range dodItems {
					items = append(items, map[string]any{
						"text": itm, "evidence": dodEvidence[i],
						"completed": true,
					})
				}
				data["dod_checklist"] = items
			}
		}
		if len(artifacts) > 0 {
			data["artifacts"] = artifacts
		}
		encoded, e := json.Marshal(data)
		if e != nil || len(encoded) > 64000 {
			fmt.Fprintln(errOut, "Report too large or invalid")
			return 2
		}
		body = bytes.NewReader(encoded)
	}
	endpoint := c.API
	if command == "push" {
		if fs.NFlag() != 0 {
			fmt.Fprintln(errOut, "task push does not accept report flags")
			return 2
		}
		method = "POST"
		endpoint = strings.TrimRight(endpoint, "/") + "/push"
	}
	req, e := http.NewRequest(method, endpoint, body)
	if e != nil {
		fmt.Fprintln(errOut, "Invalid task API")
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	timeout := 10 * time.Second
	if command == "push" {
		timeout = 65 * time.Second
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		fmt.Fprintln(errOut, "Task API unavailable; the report was not confirmed. Retry from this session.")
		return 1
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if e != nil {
		fmt.Fprintln(errOut, "Could not read task response")
		return 1
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(errOut, "Task API %d: %s\n", resp.StatusCode, bytes.TrimSpace(data))
		return 1
	}
	if command == "report-template" {
		data, e = PrepareReportTemplate(data, c)
		if e != nil {
			fmt.Fprintln(errOut, e.Error())
			return 1
		}
	}
	_, e = out.Write(data)
	if e != nil {
		return 1
	}
	fmt.Fprintln(out)
	return 0
}
