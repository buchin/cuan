package publisher

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	rxFencedBlock   = regexp.MustCompile("(?s)^```(?:[a-zA-Z0-9_-]+)?\\r?\\n(.*?)\\r?\\n```\\s*$")
	rxHeader        = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	rxHorizontalRule = regexp.MustCompile(`^(\*{3,}|-{3,}|_{3,})$`)
	rxUnorderedList = regexp.MustCompile(`^[\*\-\+]\s+(.+)$`)
	rxOrderedList   = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
	rxBlockquote    = regexp.MustCompile(`^>\s?(.*)$`)

	rxInlineCode       = regexp.MustCompile("`([^`]+)`")
	rxBoldStars        = regexp.MustCompile(`\*\*(.*?)\*\*`)
	rxBoldUnderscores  = regexp.MustCompile(`__(.*?)__`)
	rxItalicStar       = regexp.MustCompile(`\*([^*]+)\*`)
	rxItalicUnderscore = regexp.MustCompile(`_([^_]+)_`)
	rxStrike           = regexp.MustCompile(`~~(.*?)~~`)
	rxImage            = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	rxLink             = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	rxHTMLTag          = regexp.MustCompile(`<[^>]+>`)
)

// CleanMarkdown strips outer code fences (e.g. ```markdown ... ```) if an LLM wrapped its output,
// cleans null bytes, BOMs, normalizes CRLF to LF, and trims whitespace.
func CleanMarkdown(input string) string {
	cleaned := strings.TrimSpace(input)
	// Remove UTF-8 BOM if present
	cleaned = strings.TrimPrefix(cleaned, "\uFEFF")
	// Remove null bytes
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	// Normalize line endings
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")

	// If the entire text is enclosed in ```markdown ... ``` or ``` ... ```
	if matches := rxFencedBlock.FindStringSubmatch(cleaned); len(matches) > 1 {
		cleaned = strings.TrimSpace(matches[1])
	}

	return cleaned
}

// MarkdownToHTML converts markdown text into clean, semantic HTML suitable for WordPress or web rendering.
func MarkdownToHTML(input string) string {
	md := CleanMarkdown(input)
	if md == "" {
		return ""
	}

	lines := strings.Split(md, "\n")
	var htmlOut strings.Builder

	inCodeBlock := false
	codeBlockLang := ""
	var codeLines []string

	inUnorderedList := false
	inOrderedList := false
	inBlockquote := false
	var blockquoteLines []string
	var paragraphLines []string

	flushParagraph := func() {
		if len(paragraphLines) > 0 {
			pText := strings.Join(paragraphLines, "<br />\n")
			pText = renderInlineMarkdown(pText)
			htmlOut.WriteString("<p>" + pText + "</p>\n")
			paragraphLines = nil
		}
	}

	flushList := func() {
		if inUnorderedList {
			htmlOut.WriteString("</ul>\n")
			inUnorderedList = false
		}
		if inOrderedList {
			htmlOut.WriteString("</ol>\n")
			inOrderedList = false
		}
	}

	flushBlockquote := func() {
		if inBlockquote {
			bqText := strings.Join(blockquoteLines, "<br />\n")
			bqText = renderInlineMarkdown(bqText)
			htmlOut.WriteString("<blockquote><p>" + bqText + "</p></blockquote>\n")
			blockquoteLines = nil
			inBlockquote = false
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block toggle
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// Close code block
				codeContent := html.EscapeString(strings.Join(codeLines, "\n"))
				if codeBlockLang != "" {
					htmlOut.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>\n", html.EscapeString(codeBlockLang), codeContent))
				} else {
					htmlOut.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", codeContent))
				}
				inCodeBlock = false
				codeLines = nil
				codeBlockLang = ""
			} else {
				// Open code block
				flushParagraph()
				flushList()
				flushBlockquote()
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(trimmed, "```")
				codeBlockLang = strings.TrimSpace(codeBlockLang)
				codeLines = nil
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Empty line resets paragraphs, lists, blockquotes
		if trimmed == "" {
			flushParagraph()
			flushList()
			flushBlockquote()
			continue
		}

		// Horizontal rule
		if rxHorizontalRule.MatchString(trimmed) {
			flushParagraph()
			flushList()
			flushBlockquote()
			htmlOut.WriteString("<hr />\n")
			continue
		}

		// Heading
		if m := rxHeader.FindStringSubmatch(trimmed); len(m) > 2 {
			flushParagraph()
			flushList()
			flushBlockquote()
			level := len(m[1])
			hText := renderInlineMarkdown(m[2])
			htmlOut.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, hText, level))
			continue
		}

		// Blockquote
		if m := rxBlockquote.FindStringSubmatch(line); len(m) > 1 {
			flushParagraph()
			flushList()
			inBlockquote = true
			blockquoteLines = append(blockquoteLines, m[1])
			continue
		} else if inBlockquote {
			flushBlockquote()
		}

		// Unordered list item
		if m := rxUnorderedList.FindStringSubmatch(trimmed); len(m) > 1 {
			flushParagraph()
			if inOrderedList {
				flushList()
			}
			if !inUnorderedList {
				htmlOut.WriteString("<ul>\n")
				inUnorderedList = true
			}
			liText := renderInlineMarkdown(m[1])
			htmlOut.WriteString(fmt.Sprintf("  <li>%s</li>\n", liText))
			continue
		}

		// Ordered list item
		if m := rxOrderedList.FindStringSubmatch(trimmed); len(m) > 2 {
			flushParagraph()
			if inUnorderedList {
				flushList()
			}
			if !inOrderedList {
				htmlOut.WriteString("<ol>\n")
				inOrderedList = true
			}
			liText := renderInlineMarkdown(m[2])
			htmlOut.WriteString(fmt.Sprintf("  <li>%s</li>\n", liText))
			continue
		}

		// If we were in a list and this is not a list item, close list
		if inUnorderedList || inOrderedList {
			flushList()
		}

		// Regular text line (accumulate in current paragraph)
		paragraphLines = append(paragraphLines, trimmed)
	}

	// Flush any remaining active blocks
	if inCodeBlock {
		codeContent := html.EscapeString(strings.Join(codeLines, "\n"))
		htmlOut.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", codeContent))
	}
	flushParagraph()
	flushList()
	flushBlockquote()

	return strings.TrimSpace(htmlOut.String())
}

// renderInlineMarkdown transforms bold, italic, code, links, images, and strikethrough.
func renderInlineMarkdown(text string) string {
	// Images
	text = rxImage.ReplaceAllStringFunc(text, func(m string) string {
		parts := rxImage.FindStringSubmatch(m)
		if len(parts) > 2 {
			alt := html.EscapeString(parts[1])
			src := html.EscapeString(parts[2])
			return fmt.Sprintf(`<img src="%s" alt="%s" />`, src, alt)
		}
		return m
	})

	// Links
	text = rxLink.ReplaceAllStringFunc(text, func(m string) string {
		parts := rxLink.FindStringSubmatch(m)
		if len(parts) > 2 {
			title := parts[1]
			href := html.EscapeString(parts[2])
			return fmt.Sprintf(`<a href="%s">%s</a>`, href, title)
		}
		return m
	})

	// Inline code
	text = rxInlineCode.ReplaceAllStringFunc(text, func(m string) string {
		code := strings.Trim(m, "`")
		return fmt.Sprintf("<code>%s</code>", html.EscapeString(code))
	})

	// Bold (before italic)
	text = rxBoldStars.ReplaceAllString(text, "<strong>$1</strong>")
	text = rxBoldUnderscores.ReplaceAllString(text, "<strong>$1</strong>")

	// Italic
	text = rxItalicStar.ReplaceAllString(text, "<em>$1</em>")
	text = rxItalicUnderscore.ReplaceAllString(text, "<em>$1</em>")

	// Strikethrough
	text = rxStrike.ReplaceAllString(text, "<del>$1</del>")

	return text
}

// MarkdownToPlainText converts markdown content into clean plain text suitable for Teleprompter reading.
func MarkdownToPlainText(input string) string {
	cleaned := CleanMarkdown(input)
	if cleaned == "" {
		return ""
	}

	lines := strings.Split(cleaned, "\n")
	var outLines []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			outLines = append(outLines, line)
			continue
		}

		if trimmed == "" {
			outLines = append(outLines, "")
			continue
		}

		// Horizontal rule
		if rxHorizontalRule.MatchString(trimmed) {
			outLines = append(outLines, "---")
			continue
		}

		// Heading: remove leading '#'
		if m := rxHeader.FindStringSubmatch(trimmed); len(m) > 2 {
			outLines = append(outLines, m[2])
			continue
		}

		// Blockquote: remove leading '>'
		if m := rxBlockquote.FindStringSubmatch(line); len(m) > 1 {
			trimmed = strings.TrimSpace(m[1])
		}

		// Unordered list: standardize to bullet
		if m := rxUnorderedList.FindStringSubmatch(trimmed); len(m) > 1 {
			trimmed = "• " + m[1]
		}

		// Ordered list: keep number
		if m := rxOrderedList.FindStringSubmatch(trimmed); len(m) > 2 {
			trimmed = fmt.Sprintf("%s. %s", m[1], m[2])
		}

		// Strip inline images to [Image: alt]
		trimmed = rxImage.ReplaceAllString(trimmed, "[Image: $1]")

		// Strip links to "text (url)" or just "text"
		trimmed = rxLink.ReplaceAllStringFunc(trimmed, func(m string) string {
			parts := rxLink.FindStringSubmatch(m)
			if len(parts) > 2 {
				text := parts[1]
				urlStr := parts[2]
				if strings.TrimSpace(text) == strings.TrimSpace(urlStr) {
					return text
				}
				return fmt.Sprintf("%s (%s)", text, urlStr)
			}
			return m
		})

		// Strip inline code markers
		trimmed = rxInlineCode.ReplaceAllString(trimmed, "$1")

		// Strip bold/italic/strikethrough markers
		trimmed = rxBoldStars.ReplaceAllString(trimmed, "$1")
		trimmed = rxBoldUnderscores.ReplaceAllString(trimmed, "$1")
		trimmed = rxItalicStar.ReplaceAllString(trimmed, "$1")
		trimmed = rxItalicUnderscore.ReplaceAllString(trimmed, "$1")
		trimmed = rxStrike.ReplaceAllString(trimmed, "$1")

		// Strip raw HTML tags if any
		trimmed = rxHTMLTag.ReplaceAllString(trimmed, "")

		// Unescape HTML entities
		trimmed = html.UnescapeString(trimmed)

		outLines = append(outLines, trimmed)
	}

	result := strings.Join(outLines, "\n")
	// Trim trailing blank lines
	return strings.TrimSpace(result)
}

// GenerateHTMLDocument wraps the HTML body in a valid standalone HTML5 template.
func GenerateHTMLDocument(title, bodyHTML string) string {
	escapedTitle := html.EscapeString(title)
	if escapedTitle == "" {
		escapedTitle = "Document"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; max-width: 800px; margin: 40px auto; padding: 0 20px; color: #24292e; }
    h1, h2, h3, h4, h5, h6 { margin-top: 24px; margin-bottom: 16px; font-weight: 600; line-height: 1.25; }
    pre { background: #f6f8fa; padding: 16px; border-radius: 6px; overflow: auto; }
    code { font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace; font-size: 85%%; }
    blockquote { padding: 0 1em; color: #6a737d; border-left: 0.25em solid #dfe2e5; margin: 0 0 16px 0; }
    hr { height: 0.25em; padding: 0; margin: 24px 0; background-color: #e1e4e8; border: 0; }
    a { color: #0366d6; text-decoration: none; }
    a:hover { text-decoration: underline; }
  </style>
</head>
<body>
%s
</body>
</html>`, escapedTitle, bodyHTML)
}

// FormatContent formats a title and raw content into the requested export format.
// Supported formats: "md", "html", "txt".
func FormatContent(taskID, title, rawContent, format string) (*ExportResult, error) {
	normFormat := strings.ToLower(strings.TrimSpace(format))
	if normFormat == "" {
		normFormat = "md"
	}

	cleanMD := CleanMarkdown(rawContent)
	sanitizedTitle := strings.TrimSpace(title)
	if sanitizedTitle == "" {
		sanitizedTitle = "Untitled Task"
	}

	slug := sanitizeFilename(sanitizedTitle)
	if slug == "" {
		slug = "task-" + taskID
	}

	switch normFormat {
	case "md", "markdown":
		var finalMD string
		// Ensure title is present at top if not already matching first header
		firstLine := strings.Split(cleanMD, "\n")[0]
		if !strings.HasPrefix(firstLine, "# ") {
			finalMD = fmt.Sprintf("# %s\n\n%s", sanitizedTitle, cleanMD)
		} else {
			finalMD = cleanMD
		}
		return &ExportResult{
			TaskID:      taskID,
			Title:       sanitizedTitle,
			Format:      "md",
			Content:     strings.TrimSpace(finalMD),
			ContentType: "text/markdown; charset=utf-8",
			Filename:    slug + ".md",
		}, nil

	case "html":
		bodyHTML := MarkdownToHTML(cleanMD)
		// Check if h1 is present
		if !strings.HasPrefix(strings.TrimSpace(bodyHTML), "<h1") {
			bodyHTML = fmt.Sprintf("<h1>%s</h1>\n%s", html.EscapeString(sanitizedTitle), bodyHTML)
		}
		fullHTML := GenerateHTMLDocument(sanitizedTitle, bodyHTML)
		return &ExportResult{
			TaskID:      taskID,
			Title:       sanitizedTitle,
			Format:      "html",
			Content:     fullHTML,
			ContentType: "text/html; charset=utf-8",
			Filename:    slug + ".html",
		}, nil

	case "txt", "text":
		plainText := MarkdownToPlainText(cleanMD)
		var finalTxt string
		if !strings.HasPrefix(plainText, sanitizedTitle) {
			finalTxt = fmt.Sprintf("%s\n\n%s", sanitizedTitle, plainText)
		} else {
			finalTxt = plainText
		}
		return &ExportResult{
			TaskID:      taskID,
			Title:       sanitizedTitle,
			Format:      "txt",
			Content:     strings.TrimSpace(finalTxt),
			ContentType: "text/plain; charset=utf-8",
			Filename:    slug + ".txt",
		}, nil

	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedExportFormat, format)
	}
}

// sanitizeFilename produces a safe file name slug from a string.
func sanitizeFilename(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	reg := regexp.MustCompile(`[^a-z0-9_-]+`)
	slug := reg.ReplaceAllString(s, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return slug
}

// ParseTagIDs attempts to parse a slice of tag strings into numeric WordPress tag IDs.
func ParseTagIDs(tags []string) []int {
	var ids []int
	for _, t := range tags {
		trimmed := strings.TrimSpace(t)
		if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
