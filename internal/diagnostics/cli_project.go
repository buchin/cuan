package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type environmentNames []string

func (n *environmentNames) String() string { return strings.Join(*n, ",") }
func (n *environmentNames) Set(value string) error {
	if !environmentKey.MatchString(value) {
		return fmt.Errorf("invalid environment variable name")
	}
	*n = append(*n, value)
	return nil
}

// RunProjectCLI manages project environments through the authenticated Cuan API.
func RunProjectCLI(args []string, in io.Reader, out, errOut io.Writer) int {
	const help = `Cuan project environments (same settings as the application UI)

Usage:
  cuan project settings show PROJECT_ID
  cuan project settings set PROJECT_ID [--deploy-script TEXT] [--auto-deploy true|false] [--deploy-branch TEXT]
  cuan project guidance show PROJECT_ID
  cuan project guidance set PROJECT_ID --goal TEXT [--instructions TEXT]
  cuan project env list PROJECT_ID [--reveal]
  cuan project env set PROJECT_ID --from-env NAME [--from-env OTHER_NAME]
  cuan project env set PROJECT_ID --file PATH   (use --file - for stdin)
  cuan project env unset PROJECT_ID --key NAME [--key OTHER_NAME]

Output is JSON, with values redacted unless list --reveal is explicit.
Set merges selected keys; unset removes only selected keys. Other settings stay intact.
--from-env imports only named process variables; values never appear in arguments.
Files/stdin accept KEY=VALUE lines with optional surrounding quotes and # comments.
Updates require project owner access and reject concurrent settings changes.
Changes apply to new tasks; existing tasks retain their environment snapshot.`
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, help)
		return 0
	}
	if args[0] == "settings" {
		return runProjectSettingsCLI(args[1:], out, errOut)
	}
	if args[0] == "guidance" {
		return runProjectGuidanceCLI(args[1:], out, errOut)
	}
	if len(args) < 2 || args[0] != "env" {
		fmt.Fprintln(errOut, help)
		return 2
	}
	if args[1] == "--help" || args[1] == "help" {
		fmt.Fprintln(out, help)
		return 0
	}
	action := args[1]
	if action != "list" && action != "set" && action != "unset" {
		fmt.Fprintln(errOut, "Use project env list, set, or unset.")
		return 2
	}
	if len(args) < 3 || strings.HasPrefix(args[2], "-") {
		fmt.Fprintln(errOut, "project ID is required before flags")
		return 2
	}
	fs := flag.NewFlagSet("project env "+action, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var names, unset environmentNames
	var file string
	var reveal bool
	switch action {
	case "list":
		fs.BoolVar(&reveal, "reveal", false, "Explicitly include secret values in output")
	case "set":
		fs.Var(&names, "from-env", "Name of a process environment variable to import (repeatable)")
		fs.StringVar(&file, "file", "", "Read dotenv input from a file, or - for stdin")
	case "unset":
		fs.Var(&unset, "key", "Environment key to remove (repeatable)")
	}
	if err := fs.Parse(args[3:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(errOut, "Unexpected positional arguments; values must come from --from-env or --file.")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := GetCuanClient()
	var result ProjectEnvironmentResult
	var err error
	if action == "list" {
		result, err = client.GetProjectEnvironment(ctx, args[2], reveal)
	} else {
		values := map[string]string{}
		if file != "" {
			reader := in
			if file != "-" {
				f, openErr := os.Open(file)
				if openErr != nil {
					fmt.Fprintln(errOut, "Could not open environment file.")
					return 1
				}
				defer f.Close()
				reader = f
			}
			data, readErr := io.ReadAll(io.LimitReader(reader, 128001))
			if readErr != nil || len(data) > 128000 {
				fmt.Fprintln(errOut, "Could not read environment input (max 128,000 bytes).")
				return 1
			}
			values, err = parseEnvironmentInput(string(data))
			if err != nil {
				fmt.Fprintln(errOut, err)
				return 2
			}
		}
		if err = environmentFromProcess(names, values); err == nil {
			result, err = client.UpdateProjectEnvironment(ctx, args[2], values, unset)
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(errOut, "Could not write environment result.")
		return 1
	}
	return 0
}

func parseEnvironmentInput(raw string) (map[string]string, error) {
	values := map[string]string{}
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !environmentKey.MatchString(key) {
			return nil, fmt.Errorf("invalid environment assignment on line %d", i+1)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate environment key on line %d", i+1)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, nil
}
