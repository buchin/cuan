package git

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// A task/source pair keeps its verifier workspace across bounded recovery and
// finalization, while remaining separate from the developer's mutable checkout.
// Hash identities to prevent task IDs or revisions becoming filesystem paths.
func VerificationWorkspacePath(base, taskID, sourceCommit string) string {
	identity := sha256.Sum256([]byte(taskID + "\x00" + sourceCommit))
	return filepath.Join(base, "verification", fmt.Sprintf("%x", identity))
}
