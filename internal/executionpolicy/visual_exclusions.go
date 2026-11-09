package executionpolicy

import "regexp"

// Bounded negative lists describe excluded work, not requested image/frontend
// capabilities. Stop at an unknown item or a later affirmative instruction.
var negativeVisualList = regexp.MustCompile(`(?i)\b(?:no|without|tanpa|tidak\s+(?:ada|boleh)|jangan|dilarang|do not|don't|never)\s+(?:(?:take|capture|create|generate|use|run|launch|open|invoke|buat|ambil|membuat|mengambil|gunakan|menggunakan|pakai|buka|membuka|jalankan|menjalankan|ubah|mengubah|modify|modifying|edit|touch)\s+)?` + visualExclusionItem + `(?:(?:\s*,\s*(?:(?:and|or|dan|atau)\s+)?|\s+(?:and|or|dan|atau)\s+)` + visualExclusionItem + `)*\b`)

const visualExclusionItem = `(?:(?:(?:modifikasi|perubahan|changes?\s+to)\s+)?(?:ui|ux|frontend|front-end)(?:\s*/\s*(?:ui|ux|frontend|front-end))*(?:\s+changes?)?|browser(?:(?:\s*/\s*|\s+)(?:automation|screenshots?))?|screenshots?|tangkapan\s+layar)`

var excludedCaptureItem = regexp.MustCompile(`(?i)\b(?:browser|screenshots?|tangkapan\s+layar)\b`)

// WithoutVisualExclusions retains positive requirements outside explicit lists.
func WithoutVisualExclusions(text string) string {
	return negativeVisualList.ReplaceAllString(text, " ")
}

// ExcludesVisualCapture does not mistake a no-UI-changes restriction for a ban
// on capturing an existing UI. Reading supplied images remains independent.
func ExcludesVisualCapture(text string) bool {
	for _, excluded := range negativeVisualList.FindAllString(text, -1) {
		if excludedCaptureItem.MatchString(excluded) {
			return true
		}
	}
	return false
}
