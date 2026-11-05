package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type resendPublicSettings struct {
	HasWebhookSecret     bool       `json:"has_webhook_secret"`
	HasAPIKey            bool       `json:"has_api_key"`
	FromEmail            string     `json:"from_email"`
	FromName             string     `json:"from_name"`
	NotificationEmail    string     `json:"notification_email"`
	Enabled              bool       `json:"enabled"`
	ValidationStatus     string     `json:"validation_status"`
	LastValidatedAt      *time.Time `json:"last_validated_at,omitempty"`
	IdleThresholdMinutes int        `json:"idle_threshold_minutes"`
}
type resendSettingsResult struct {
	Settings resendPublicSettings `json:"settings"`
}
type resendValidationResult struct {
	Valid  bool   `json:"valid"`
	Status string `json:"validation_status"`
}

// RunIntegrationCLI uses authenticated Settings endpoints. Secrets are imported
// only by explicitly named env variables; neither request bodies nor API errors
// are printed. It never sends test mail or changes automatic notification policy.
func RunIntegrationCLI(args []string, out, errOut io.Writer) int {
	const help = `Usage:
	cuan integration github show
  cuan integration resend show
  cuan integration resend set [--key-from-env NAME] [--webhook-secret-from-env NAME] [--from-email EMAIL] [--from-name NAME] [--notification-email EMAIL]
  cuan integration resend validate
Set preserves automatic-email preferences and other omitted settings.
Secret values are never accepted as arguments or returned in output.
Validate checks credentials without sending email.`
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, help)
		return 0
	}
	if args[0] == "github" {
		if len(args) != 2 || args[1] != "show" {
			fmt.Fprintln(errOut, help)
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		status, err := GetCuanClient().GitHubConnection(ctx)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if json.NewEncoder(out).Encode(status) != nil {
			return 1
		}
		return 0
	}
	if len(args) < 2 || args[0] != "resend" {
		fmt.Fprintln(errOut, help)
		return 2
	}
	fs := flag.NewFlagSet("integration resend", flag.ContinueOnError)
	fs.SetOutput(errOut)
	keyEnv := fs.String("key-from-env", "", "Explicit secret environment variable")
	webhookEnv := fs.String("webhook-secret-from-env", "", "Explicit Resend webhook signing secret variable for this Cuan installation")
	from := fs.String("from-email", "", "Sender address")
	name := fs.String("from-name", "", "Sender name")
	notify := fs.String("notification-email", "", "Notification address")
	if err := fs.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || (args[1] != "set" && fs.NFlag() != 0) {
		fmt.Fprintln(errOut, help)
		return 2
	}
	client := GetCuanClient()
	if !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure Cuan API URL and auth token first.")
		return 1
	}
	method, endpoint := http.MethodGet, "/api/v1/settings/resend"
	var body any
	var result any = &resendSettingsResult{}
	switch args[1] {
	case "show":
	case "set":
		payload := map[string]string{}
		if *webhookEnv != "" {
			if !environmentKey.MatchString(*webhookEnv) || os.Getenv(*webhookEnv) == "" {
				fmt.Fprintln(errOut, "Named webhook secret variable is invalid or empty.")
				return 2
			}
			payload["webhook_secret"] = os.Getenv(*webhookEnv)
		}
		if *keyEnv != "" {
			if !environmentKey.MatchString(*keyEnv) {
				fmt.Fprintln(errOut, "Invalid secret variable name.")
				return 2
			}
			value := os.Getenv(*keyEnv)
			if value == "" {
				fmt.Fprintln(errOut, "Named secret environment variable is empty.")
				return 2
			}
			payload["api_key"] = value
		}
		if *from != "" {
			payload["from_email"] = *from
		}
		if *name != "" {
			payload["from_name"] = *name
		}
		if *notify != "" {
			payload["notification_email"] = *notify
		}
		if len(payload) == 0 {
			fmt.Fprintln(errOut, "Specify a setting to update.")
			return 2
		}
		method = http.MethodPost
		body = payload
	case "validate":
		method = http.MethodPost
		endpoint += "/validate"
		body = map[string]string{}
		result = &resendValidationResult{}
	default:
		fmt.Fprintln(errOut, help)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.doRequest(ctx, method, endpoint, body)
	if err != nil {
		fmt.Fprintln(errOut, "Integration request failed.")
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(errOut, "Integration request failed (HTTP %d).\n", resp.StatusCode)
		return 1
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(result); err != nil {
		fmt.Fprintln(errOut, "Invalid integration response.")
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return 1
	}
	if v, ok := result.(*resendValidationResult); ok && !v.Valid {
		return 1
	}
	return 0
}
