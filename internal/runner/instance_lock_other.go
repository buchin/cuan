//go:build !linux && !darwin

package runner

// AcquireRunnerInstanceLock is a no-op on platforms without flock.
func AcquireRunnerInstanceLock(_, _ string) (func(), error) {
	return func() {}, nil
}
