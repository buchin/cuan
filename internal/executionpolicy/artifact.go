package executionpolicy

import (
	"regexp"
	"strings"
)

var standaloneWork = regexp.MustCompile(`(?i)\b(?:standalone|stand-alone|mandiri)\b`)
var artifactWork = regexp.MustCompile(`(?i)\b(?:files?|berkas|attachments?|lampiran|downloads?|unduh)\b`)
var noRepositoryChanges = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|must not|never|jangan|tidak(?: perlu)?|tanpa|without)\s+(?:(?:open|read|inspect|membuka|buka|membaca|baca|mengakses|akses)\s+(?:or|and|atau|dan)\s+)?(?:change|modify|modifying|edit|touch|mengubah|ubah|mengedit|modifikasi)\s+(?:(?:the|any|existing|project|source)\s+){0,2}(?:repo|repository|repositories|repositori)|no\s+(?:repo|repository)\s+changes|(?:zero|no)\s+repository\s+mutation)\b`)

// StandaloneArtifactWork is a narrow execution requirement, not publication
// authority. It requires explicit independent file work, no repository changes
// and no deployment. Project defaults cannot turn this into a coding task.
func StandaloneArtifactWork(instructions string) bool {
	scope := publicationScope(instructions)
	return standaloneArtifactShape(scope) && FromInstructions(scope).DenyDeploy
}
func standaloneArtifactShape(scope string) bool {
	return standaloneWork.MatchString(scope) && artifactWork.MatchString(scope) && noRepositoryChanges.MatchString(scope)
}
func publicationScope(instructions string) string {
	instructions = NormalizeManagedInstructions(instructions)
	for _, marker := range []string{"\nProject guidance snapshot:", "\nProject guidance snapshot (context only; does not expand task authorization):"} {
		if i := strings.Index(instructions, marker); i >= 0 {
			instructions = instructions[:i]
		}
	}
	return instructions
}
