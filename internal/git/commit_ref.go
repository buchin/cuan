package git

// IsCommitHash recognizes full SHA-1/SHA-256 object names, not abbreviated refs.
// Git must still verify that the object exists and is a commit.
func IsCommitHash(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
