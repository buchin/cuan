package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var cliSettingsMu sync.Mutex

// prepareAntigravitySettings uses the provider's documented settings.json keys
// before first sign-in. It never records acceptance of provider terms or opts in
// to interaction sharing. Existing explicit preferences remain unchanged.
// Reference: https://antigravity.google/docs/cli/reference#configuration-keys-settingsjson
func prepareAntigravitySettings(home string) error {
	cliSettingsMu.Lock()
	defer cliSettingsMu.Unlock()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	path := filepath.Join(dir, "settings.json")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	settings := map[string]json.RawMessage{}
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("CLI settings need review; existing settings were preserved")
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if json.Unmarshal(data, &settings) != nil || settings == nil {
			return errors.New("CLI settings could not be read; existing settings were preserved")
		}
	}
	changed := false
	for key, value := range map[string]json.RawMessage{"colorScheme": json.RawMessage(`"terminal"`), "enableTelemetry": json.RawMessage(`false`)} {
		if _, exists := settings[key]; !exists {
			settings[key] = value
			changed = true
		}
	}
	if !changed {
		return nil
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
