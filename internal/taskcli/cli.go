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
	API       string `json:"api"`
	Token     string `json:"token"`
	TaskID    string `json:"task_id"`
	AttemptID string `json:"attempt_id"`
}

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, "Usage: cuan-server task get | report [--title TEXT] [--priority low|normal|high|urgent] [--summary TEXT] [--complete-note TEXT] [--progress TEXT] [--state progress|review|blocked] [--result-kind answer|implementation] [--report-file FILE]\nUses CUAN_TASK_CAPABILITY_FILE; reports affect only the current task/session. Human instructions and Done cannot be changed. Review requires summary and verification evidence; workflow changes after process exit.")
		fmt.Fprintln(out, `Optional --report-file JSON example:
{"state":"review","summary":"Actual result","complete_note":"Verification and limitations","dod_checklist":[{"text":"Verified requirement","completed":true}],"commit_sha":"optional commit","test_summary":"Actual checks"}
Omit metadata/checklists when unnecessary. Use --summary and --complete-note directly for a simple result.`)
		return 0
	}
	command := args[0]
	if command != "get" && command != "report" {
		fmt.Fprintln(errOut, "Unknown task command; use task --help")
		return 2
	}
	fs := flag.NewFlagSet("task "+command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	title := fs.String("title", "", "Refined task title")
	priority := fs.String("priority", "", "Task priority")
	summary := fs.String("summary", "", "Work plan or result summary")
	note := fs.String("complete-note", "", "Actual verification evidence or blocker")
	progress := fs.String("progress", "", "Progress checkpoint")
	state := fs.String("state", "", "progress, review or blocked")
	kind := fs.String("result-kind", "", "answer for conversation without file changes; implementation otherwise")
	file := fs.String("report-file", "", "Structured JSON report file (never human instructions)")
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
	path := os.Getenv("CUAN_TASK_CAPABILITY_FILE")
	info, e := os.Stat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		fmt.Fprintln(errOut, "Task capability is unavailable or not private. Run this tool inside the bound agent session.")
		return 1
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		fmt.Fprintln(errOut, "Cannot read task capability")
		return 1
	}
	var c Capability
	if e = json.Unmarshal(raw, &c); e != nil || c.Token == "" || c.TaskID == "" {
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
		encoded, e := json.Marshal(data)
		if e != nil || len(encoded) > 64000 {
			fmt.Fprintln(errOut, "Report too large or invalid")
			return 2
		}
		body = bytes.NewReader(encoded)
	}
	req, e := http.NewRequest(method, c.API, body)
	if e != nil {
		fmt.Fprintln(errOut, "Invalid task API")
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
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
	_, e = out.Write(data)
	if e != nil {
		return 1
	}
	fmt.Fprintln(out)
	return 0
}
