package server

import (
	"fmt"
	"io"
	"mime"
	"os"
	"strings"
	"unicode/utf8"
)

// Structured text uses application/* MIME types too. Do not guess binary
// contents from filenames; decoding still requires valid UTF-8 below.
func reviewableTextMediaType(value string) bool {
	kind, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	return strings.HasPrefix(kind, "text/") || kind == "application/json" ||
		kind == "application/xml" || (strings.HasPrefix(kind, "application/") &&
		(strings.HasSuffix(kind, "+json") || strings.HasSuffix(kind, "+xml")))
}

// Reports commonly put checks and limitations after their introduction. Preserve
// both ends inside the existing context budget and mark the omitted middle.
func reviewTextExcerpt(path string, budget int) (text, scope string, err error) {
	if budget <= 0 {
		return "", "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("review text is unavailable")
	}
	const omitted = "\n[... middle omitted ...]\n"
	if info.Size() <= int64(budget) {
		data, err := io.ReadAll(io.LimitReader(f, int64(budget)))
		if err != nil || !utf8.Valid(data) {
			return "", "", fmt.Errorf("review text could not be decoded")
		}
		return string(data), "complete", nil
	}
	if budget <= len(omitted)+2*utf8.UTFMax {
		return "", "", nil
	}
	headSize := (budget - len(omitted)) / 2
	tailSize := budget - len(omitted) - headSize
	head := make([]byte, headSize)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", "", err
	}
	if _, err := f.Seek(info.Size()-int64(tailSize), io.SeekStart); err != nil {
		return "", "", err
	}
	tail := make([]byte, tailSize)
	if _, err := io.ReadFull(f, tail); err != nil {
		return "", "", err
	}
	for i := 0; i < utf8.UTFMax && len(head) > 0 && !utf8.Valid(head); i++ {
		head = head[:len(head)-1]
	}
	for i := 0; i < utf8.UTFMax && len(tail) > 0 && !utf8.Valid(tail); i++ {
		tail = tail[1:]
	}
	if !utf8.Valid(head) || !utf8.Valid(tail) {
		return "", "", fmt.Errorf("review text could not be decoded")
	}
	return string(head) + omitted + string(tail), "head_and_tail", nil
}
