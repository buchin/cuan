package release

import "sync"

// Release scripts may emit these exact lines. They are progress hints only,
// never evidence of successful execution, deployment or task acceptance.
const progressPrefix = "::cuan-release-stage::"

var progressStages = map[string]string{
	"checks":       "Checking release safety",
	"dependencies": "Preparing release dependencies",
	"tests":        "Running release tests",
	"build":        "Building release artifacts",
	"transfer":     "Transferring release artifacts",
	"activate":     "Activating release",
	"health":       "Checking production health",
}

// Only bounded, allowlisted milestones cross this boundary. Raw child output
// (including secrets split across writes) is never forwarded to the observer.
type progressWriter struct {
	mu      sync.Mutex
	line    []byte
	discard bool
	seen    map[string]bool
	observe func(string)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			if !w.discard {
				stage := string(w.line)
				if label, ok := progressStages[trimProgressPrefix(stage)]; ok && !w.seen[stage] {
					if w.seen == nil {
						w.seen = make(map[string]bool)
					}
					w.seen[stage] = true
					if w.observe != nil {
						w.observe(label)
					}
				}
			}
			w.line, w.discard = w.line[:0], false
		} else if !w.discard {
			if len(w.line) >= 128 {
				w.line, w.discard = w.line[:0], true
			} else {
				w.line = append(w.line, b)
			}
		}
	}
	return len(p), nil
}

func trimProgressPrefix(line string) string {
	if len(line) < len(progressPrefix) || line[:len(progressPrefix)] != progressPrefix {
		return ""
	}
	return line[len(progressPrefix):]
}
