package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var runtimeToolName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,79}$`)

// CheckRuntime inspects the explicitly selected execution PATH only. It never
// executes tools or borrows the maintainer process PATH/profile/credentials.
func CheckRuntime(plan *Plan, environment []string) error {
	if plan == nil || environment == nil {
		return errors.New("release plan and explicit runtime environment required")
	}
	selectedPath := ""
	for _, entry := range environment {
		if strings.HasPrefix(entry, "PATH=") {
			selectedPath = strings.TrimPrefix(entry, "PATH=")
		}
	}
	for _, tool := range plan.spec.RequiredTools {
		found := false
		for _, directory := range filepath.SplitList(selectedPath) {
			if !filepath.IsAbs(directory) {
				continue
			}
			info, err := os.Stat(filepath.Join(directory, tool))
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required release tool unavailable in selected runtime: %s", tool)
		}
	}
	return nil
}
