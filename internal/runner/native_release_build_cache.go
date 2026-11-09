package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Authentication/configuration stay in the prepared release HOME. Only generated
// compiler/download caches reuse the existing owner lifecycle and maintenance lease.
func prepareNativeReleaseBuildCaches(ctx context.Context, execution *NativeReleaseExecution, workspaces, owner string) ([]string, func(), error) {
	closeAll := func() {}
	if execution == nil || execution.Workspace == nil {
		return nil, closeAll, errors.New("native release workspace missing")
	}
	if workspaces == "" {
		return execution.Environment, closeAll, nil
	}
	if owner == "" {
		return nil, closeAll, errors.New("native release cache owner missing")
	}
	base := filepath.Dir(filepath.Clean(workspaces))
	goEnv, err := managedGoBuildEnvironment(execution.Workspace.Path, filepath.Join(base, "go-build-runtime"), owner)
	if err != nil {
		return nil, closeAll, err
	}
	npmEnv, err := managedNPMBuildEnvironment(execution.Workspace.Path, filepath.Join(base, "npm-build-runtime"), owner)
	if err != nil {
		return nil, closeAll, err
	}
	leases := []*os.File{}
	closeAll = func() {
		for _, lease := range leases {
			_ = lease.Close()
		}
	}
	for _, cache := range []string{goEnv["GOCACHE"], npmEnv["NPM_CONFIG_CACHE"]} {
		lease, err := acquireBuildCacheLease(ctx, cache)
		if err != nil {
			closeAll()
			return nil, func() {}, err
		}
		if lease != nil {
			leases = append(leases, lease)
		}
	}
	selected := map[string]string{}
	for _, entry := range execution.Environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			selected[key] = value
		}
	}
	for key, value := range goEnv {
		selected[key] = value
	}
	for key, value := range npmEnv {
		selected[key] = value
	}
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+selected[key])
	}
	return environment, closeAll, nil
}
