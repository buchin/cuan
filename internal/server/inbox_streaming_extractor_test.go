package server

import (
	"strings"
	"testing"
)

func TestStreamReplyExtractor_StructuredJSON(t *testing.T) {
	extractor := newStreamReplyExtractor()
	chunks := []string{
		`{"thought": "thinking carefully...", `,
		`"topic_title": "Project Update", `,
		`"reply": "Halo `,
		`Budi!\n\nAda yang `,
		`bisa saya bantu?\", dia bertanya.`,
		`", "action": null}`,
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(extractor.Feed(chunk))
	}

	expected := "Halo Budi!\n\nAda yang bisa saya bantu?\", dia bertanya."
	if sb.String() != expected {
		t.Fatalf("expected %q, got %q", expected, sb.String())
	}
}

func TestStreamReplyExtractor_TrailingBackslashAcrossChunks(t *testing.T) {
	extractor := newStreamReplyExtractor()
	chunks := []string{
		`{"reply": "Baris 1\`,
		`nBaris 2"}`,
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(extractor.Feed(chunk))
	}

	expected := "Baris 1\nBaris 2"
	if sb.String() != expected {
		t.Fatalf("expected %q, got %q", expected, sb.String())
	}
}

func TestStreamReplyExtractor_PlainTextFallback(t *testing.T) {
	extractor := newStreamReplyExtractor()
	chunks := []string{
		"Halo, ini adalah ",
		"plain text response ",
		"tanpa wrapper JSON.",
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(extractor.Feed(chunk))
	}

	expected := "Halo, ini adalah plain text response tanpa wrapper JSON."
	if sb.String() != expected {
		t.Fatalf("expected %q, got %q", expected, sb.String())
	}
}

func TestStreamReplyExtractor_EmptyChunks(t *testing.T) {
	extractor := newStreamReplyExtractor()
	if out := extractor.Feed(""); out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
	if out := extractor.Feed("   "); out != "" {
		t.Fatalf("expected empty before JSON/text, got %q", out)
	}
}

func TestStreamReplyExtractor_CodeFenceWrapped(t *testing.T) {
	extractor := newStreamReplyExtractor()
	chunks := []string{
		"```json\n",
		`{"reply": "Halo `,
		`dunia!"}`,
		"\n```",
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(extractor.Feed(chunk))
	}

	expected := "Halo dunia!"
	if sb.String() != expected {
		t.Fatalf("expected %q, got %q", expected, sb.String())
	}
	if !extractor.hasEmitted() {
		t.Fatal("expected hasEmitted() to be true")
	}
}

func TestStreamReplyExtractor_NoLeakOfTrailingJSON(t *testing.T) {
	extractor := newStreamReplyExtractor()
	chunks := []string{
		`{"reply": "Teks jawaban", "action": {"kind": "approval", "payload": {"title": "Task"}}, "topic_title": "Judul"}`,
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(extractor.Feed(chunk))
	}

	expected := "Teks jawaban"
	if sb.String() != expected {
		t.Fatalf("expected %q, got %q", expected, sb.String())
	}
	// Subsequent calls must return empty
	if out := extractor.Feed(`{"extra": "data"}`); out != "" {
		t.Fatalf("expected empty after finished, got %q", out)
	}
}
