package agent

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var setupANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var checkedSharing = regexp.MustCompile(`\[x\]\s*Yes, I agree`)
var uncheckedSharing = regexp.MustCompile(`\[ \]\s*Yes, I agree`)

// Drives only the observed first-run wizard, before any user prompt is sent.
// Terms need explicit consent. Interaction sharing is always deselected before
// confirming. Unknown/new screens receive no input and cannot establish readiness.
type cliSetupDriver struct {
	mu            sync.Mutex
	writer        io.Writer
	text          string
	stage         int
	themeSelected bool
	accepted      bool
}

func (d *cliSetupDriver) feed(chunk string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.text += chunk
	if len(d.text) > 32768 {
		d.text = d.text[len(d.text)-16384:]
	}
	d.advance()
}
func (d *cliSetupDriver) accept() { d.mu.Lock(); defer d.mu.Unlock(); d.accepted = true; d.advance() }
func (d *cliSetupDriver) send(keys string, next int) {
	if _, err := io.WriteString(d.writer, keys); err != nil {
		return
	}
	d.text = ""
	d.stage = next
}
func (d *cliSetupDriver) advance() {
	plain := setupANSI.ReplaceAllString(d.text, "")
	compact := strings.Join(strings.Fields(plain), " ")
	switch d.stage {
	case 0:
		if !d.themeSelected && strings.Contains(plain, "Choose your color scheme:") {
			d.themeSelected = true
			d.send("\r", 0)
			return
		}
		if !d.accepted || !strings.Contains(plain, "Terms of Service & Data Use") || !strings.Contains(plain, "[Done]") {
			return
		}
		if checkedSharing.MatchString(plain) {
			d.send(" ", 1)
		} else if uncheckedSharing.MatchString(plain) {
			d.send("\t", 2)
		}
	case 1:
		// Bubble Tea can redraw just the changed checkbox cells, not a full page.
		if strings.Contains(plain, " ]") || uncheckedSharing.MatchString(plain) {
			d.send("\t", 2)
		}
	case 2:
		if strings.Contains(compact, "> Previous") {
			d.send("\t", 3)
		}
	case 3:
		if strings.Contains(compact, "> Done") {
			d.send("\r", 4)
		}
	}
}

func antigravityOnboardingComplete(home string) bool {
	data, err := os.ReadFile(filepath.Join(AntigravityConfigDirPath(home), "cache", "onboarding.json"))
	if err != nil {
		return false
	}
	var state struct {
		Complete bool `json:"onboardingComplete"`
	}
	return json.Unmarshal(data, &state) == nil && state.Complete
}

// AntigravityOnboardingComplete reports whether onboardingComplete=true is set in the profile cache.
func AntigravityOnboardingComplete(home string) bool {
	return antigravityOnboardingComplete(home)
}

// EnsureAntigravityOnboarding ensures that cache/onboarding.json is populated with
// onboardingComplete=true if the profile has valid saved credentials.
func EnsureAntigravityOnboarding(home string) error {
	var loggedIn bool
	if home != "" {
		loggedIn = IsAntigravityLoggedInInDir(home)
	} else {
		loggedIn = IsAntigravityLoggedIn()
	}
	if !loggedIn {
		return nil
	}
	if antigravityOnboardingComplete(home) {
		return nil
	}
	dir := filepath.Join(AntigravityConfigDirPath(home), "cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "onboarding.json")
	state := map[string]bool{
		"consumerOnboardingComplete":   true,
		"enterpriseOnboardingComplete": false,
		"onboardingComplete":           true,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
