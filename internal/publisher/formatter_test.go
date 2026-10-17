package publisher

import (
	"strings"
	"testing"
)

func TestCleanMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "wrapped in ```markdown",
			input:    "```markdown\n# Title\nThis is text\n```",
			expected: "# Title\nThis is text",
		},
		{
			name:     "wrapped in plain ```",
			input:    "```\n# Plain Fenced\nContent\n```",
			expected: "# Plain Fenced\nContent",
		},
		{
			name:     "contains BOM and CRLF",
			input:    "\uFEFF## Heading\r\nLine 1\r\nLine 2",
			expected: "## Heading\nLine 1\nLine 2",
		},
		{
			name:     "contains null bytes",
			input:    "Clean\x00Text",
			expected: "CleanText",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("CleanMarkdown() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMarkdownToHTML(t *testing.T) {
	input := `# Article Title

This is a paragraph with **bold** and *italic* and ~~strike~~ text.

Here is an [External Link](https://example.com) and an image ![Logo](https://example.com/logo.png).

## Subheading

- Item 1
- Item 2
- Item 3

1. Step one
2. Step two

> This is an important quote.

` + "```go\nfunc main() {\n\tprintln(\"Hello\")\n}\n```" + `

---

Inline ` + "`code_snippet()`" + ` is also supported.`

	htmlOut := MarkdownToHTML(input)

	expectedFragments := []string{
		"<h1>Article Title</h1>",
		"<h2>Subheading</h2>",
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<del>strike</del>",
		`<a href="https://example.com">External Link</a>`,
		`<img src="https://example.com/logo.png" alt="Logo" />`,
		"<ul>",
		"<li>Item 1</li>",
		"</ul>",
		"<ol>",
		"<li>Step one</li>",
		"</ol>",
		"<blockquote><p>This is an important quote.</p></blockquote>",
		`<pre><code class="language-go">func main() {`,
		"<hr />",
		"<code>code_snippet()</code>",
	}

	for _, frag := range expectedFragments {
		if !strings.Contains(htmlOut, frag) {
			t.Errorf("MarkdownToHTML() missing expected fragment %q\nFull output:\n%s", frag, htmlOut)
		}
	}
}

func TestMarkdownToPlainText(t *testing.T) {
	input := `# Script for Teleprompter

Welcome to **Cuan Studio**, the best *modular* OS!

Here are our 3 key points:
- Point A: Automated Deployment
- Point B: Safe Onboarding
- Point C: 1-Click Publishing

> Remember to smile and speak clearly.

Check out [Cuan Website](https://cuan.dojo.cc) for more info.`

	plain := MarkdownToPlainText(input)

	if strings.Contains(plain, "#") {
		t.Errorf("plain text should not contain markdown headers: %s", plain)
	}
	if strings.Contains(plain, "**") || strings.Contains(plain, "*") {
		t.Errorf("plain text should not contain bold/italic asterisks: %s", plain)
	}
	if strings.Contains(plain, ">") {
		t.Errorf("plain text should not contain blockquote indicators: %s", plain)
	}
	if !strings.Contains(plain, "Cuan Website (https://cuan.dojo.cc)") {
		t.Errorf("expected clean link representation: %s", plain)
	}
	if !strings.Contains(plain, "• Point A: Automated Deployment") {
		t.Errorf("expected clean bullet for teleprompter: %s", plain)
	}
}

func TestFormatContent(t *testing.T) {
	title := "How to Build a SaaS"
	content := "This is the **full** article content.\n\nEnjoy building!"

	// Test Markdown
	mdRes, err := FormatContent("task-1", title, content, "md")
	if err != nil {
		t.Fatalf("FormatContent(md) failed: %v", err)
	}
	if mdRes.Format != "md" || mdRes.ContentType != "text/markdown; charset=utf-8" {
		t.Errorf("unexpected md result: %+v", mdRes)
	}
	if !strings.HasPrefix(mdRes.Content, "# How to Build a SaaS") {
		t.Errorf("expected title header in markdown: %s", mdRes.Content)
	}

	// Test HTML
	htmlRes, err := FormatContent("task-1", title, content, "html")
	if err != nil {
		t.Fatalf("FormatContent(html) failed: %v", err)
	}
	if htmlRes.Format != "html" || htmlRes.ContentType != "text/html; charset=utf-8" {
		t.Errorf("unexpected html result: %+v", htmlRes)
	}
	if !strings.Contains(htmlRes.Content, "<title>How to Build a SaaS</title>") {
		t.Errorf("expected title in html template: %s", htmlRes.Content)
	}
	if !strings.Contains(htmlRes.Content, "<strong>full</strong>") {
		t.Errorf("expected rendered html body: %s", htmlRes.Content)
	}

	// Test Plain Text
	txtRes, err := FormatContent("task-1", title, content, "txt")
	if err != nil {
		t.Fatalf("FormatContent(txt) failed: %v", err)
	}
	if txtRes.Format != "txt" || txtRes.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("unexpected txt result: %+v", txtRes)
	}
	if strings.Contains(txtRes.Content, "**") {
		t.Errorf("plain text must not contain markdown asterisks: %s", txtRes.Content)
	}

	// Test unsupported format
	_, err = FormatContent("task-1", title, content, "pdf")
	if err == nil {
		t.Errorf("expected error for unsupported format 'pdf'")
	}
}
