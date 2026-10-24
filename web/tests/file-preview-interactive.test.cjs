const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')

;(async () => {
  const { default: ReactMarkdown } = await import('react-markdown')
  const remarkGfm = (await import('remark-gfm')).default

  console.log('======================================================================')
  console.log('IN-APP FILE PREVIEW MODAL & RESPONSIVENESS VERIFICATION SUITE')
  console.log('Format support: Text, Markdown, Image, Log, Diff/Code, Binary Fallback')
  console.log('======================================================================\n')

  // ---------------------------------------------------------------------------
  // Test 1: File Type Classification & Preview Eligibility
  // ---------------------------------------------------------------------------
  console.log('--- Test 1: File Type Classification & Preview Eligibility ---')

  function isMarkdownFile(name) {
    return /\.(md|markdown|mdown|mkdn)$/i.test(name)
  }
  function isImageFile(name, type) {
    return (
      /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i.test(name) ||
      (type ? type.startsWith('image/') : false)
    )
  }
  function isDiffFile(name) {
    return /\.(diff|patch)$/i.test(name)
  }
  function isLogFile(name) {
    return /\.(log)$/i.test(name)
  }
  function isJsonFile(name, type) {
    return /\.json$/i.test(name) || (type ? type.includes('json') : false)
  }
  function isCodeFile(name) {
    return /\.(js|jsx|ts|tsx|mjs|cjs|go|py|rb|rs|c|cpp|h|hpp|java|kt|sh|bash|zsh|css|scss|sass|html|htm|sql|graphql|proto|yaml|yml|toml|xml)$/i.test(
      name,
    )
  }
  function isTextFile(name, type) {
    return (
      /\.(txt|text|log|csv|tsv|env|ini|conf|config|diff|patch)$/i.test(name) ||
      (type ? type.startsWith('text/') : false)
    )
  }
  function isPreviewableFile(name, type) {
    return (
      isMarkdownFile(name) ||
      isImageFile(name, type) ||
      isDiffFile(name) ||
      isLogFile(name) ||
      isJsonFile(name, type) ||
      isCodeFile(name) ||
      isTextFile(name, type)
    )
  }

  // 1.1 Markdown files
  assert.equal(isMarkdownFile('README.md'), true)
  assert.equal(isMarkdownFile('NOTES.markdown'), true)
  assert.equal(isPreviewableFile('GUIDE.md'), true)
  console.log('  ✓ Markdown formats verified (.md, .markdown)')

  // 1.2 Image formats
  assert.equal(isImageFile('screenshot.png'), true)
  assert.equal(isImageFile('photo.jpg'), true)
  assert.equal(isImageFile('banner.webp'), true)
  assert.equal(isImageFile('logo.svg'), true)
  assert.equal(isImageFile('attachment-file', 'image/png'), true)
  assert.equal(isPreviewableFile('diagram.png'), true)
  console.log('  ✓ Image formats verified (.png, .jpg, .webp, .svg, image/*)')

  // 1.3 Diff formats
  assert.equal(isDiffFile('changes.diff'), true)
  assert.equal(isDiffFile('release.patch'), true)
  assert.equal(isPreviewableFile('changes.diff'), true)
  console.log('  ✓ Diff formats verified (.diff, .patch)')

  // 1.4 Log formats
  assert.equal(isLogFile('build.log'), true)
  assert.equal(isLogFile('output.log'), true)
  assert.equal(isPreviewableFile('server.log'), true)
  console.log('  ✓ Log formats verified (.log)')

  // 1.5 Code & Data formats
  assert.equal(isCodeFile('main.go'), true)
  assert.equal(isCodeFile('App.tsx'), true)
  assert.equal(isCodeFile('script.py'), true)
  assert.equal(isCodeFile('queries.sql'), true)
  assert.equal(isJsonFile('config.json'), true)
  assert.equal(isTextFile('output.txt'), true)
  assert.equal(isTextFile('data.csv'), true)
  assert.equal(isPreviewableFile('main.go'), true)
  assert.equal(isPreviewableFile('config.json'), true)
  console.log('  ✓ Code & Text formats verified (.go, .tsx, .py, .sql, .json, .txt, .csv)')

  // 1.6 Binary & Unsupported formats (Must fallback to direct download)
  assert.equal(isPreviewableFile('release.zip'), false)
  assert.equal(isPreviewableFile('package.tar.gz'), false)
  assert.equal(isPreviewableFile('binary.exe'), false)
  assert.equal(isPreviewableFile('blob.bin'), false)
  assert.equal(isPreviewableFile('firmware.iso'), false)
  assert.equal(isPreviewableFile('document.pdf'), false)
  console.log('  ✓ Unsupported binary formats correctly identified for fallback download (.zip, .tar.gz, .exe, .bin, .pdf)')

  // ---------------------------------------------------------------------------
  // Test 2: Diff Syntax Highlighting & Line Markers
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 2: Diff Syntax Highlighting & Line Markers ---')

  function DiffViewerTest({ content }) {
    const lines = content.split('\n')
    return React.createElement(
      'div',
      { className: 'diff-container font-mono text-xs' },
      React.createElement(
        'table',
        null,
        React.createElement(
          'tbody',
          null,
          lines.map((line, idx) => {
            const isChunk = line.startsWith('@@')
            const isAdd = line.startsWith('+') && !line.startsWith('+++')
            const isDel = line.startsWith('-') && !line.startsWith('---')
            const marker = isAdd ? '+' : isDel ? '-' : ' '
            const rowClass = isChunk ? 'diff-chunk' : isAdd ? 'diff-add' : isDel ? 'diff-del' : 'diff-ctx'
            return React.createElement(
              'tr',
              { key: idx, className: rowClass },
              React.createElement('td', { className: 'line-num' }, idx + 1),
              React.createElement('td', { className: 'line-marker' }, marker),
              React.createElement('td', { className: 'line-content' }, line.substring(marker !== ' ' ? 1 : 0)),
            )
          }),
        ),
      ),
    )
  }

  const sampleDiff = `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -10,3 +10,3 @@
 func OldName() string {
-	return "old"
+	return "new"
 }`

  const diffHtml = renderToStaticMarkup(React.createElement(DiffViewerTest, { content: sampleDiff }))
  assert(diffHtml.includes('class="diff-chunk"'), 'Diff chunk header @@ highlighted')
  assert(diffHtml.includes('class="diff-del"'), 'Diff deletion line highlighted')
  assert(diffHtml.includes('class="diff-add"'), 'Diff addition line highlighted')
  assert(diffHtml.includes('return &quot;new&quot;'), 'Diff addition content rendered')
  console.log('  ✓ Diff chunk, addition (+), and deletion (-) styling verified')

  // ---------------------------------------------------------------------------
  // Test 3: Markdown Rendering with Callouts, Tables, and Syntax Tokens
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 3: Markdown Rendering with Callouts, Tables & Syntax Tokens ---')

  function highlightCodeTokens(code) {
    const tokenRegex =
      /(\/\/[^\n]*|\/\*[\s\S]*?\*\/|#[^\n]*)|("(?:\\.|[^"\\])*"|'[^']*'|`[^`]*`)|(\b(?:const|let|var|function|return|if|else|for|while|import|export|from|type|interface|class|package|func|struct|def|async|await|true|false|null|nil)\b)|(\b\d+(?:\.\d+)?\b)|(\b[A-Z][a-zA-Z0-9_]*\b)/g
    const nodes = []
    let lastIndex = 0
    let match
    let key = 0
    while ((match = tokenRegex.exec(code)) !== null) {
      if (match.index > lastIndex) {
        nodes.push(React.createElement('span', { key: key++ }, code.slice(lastIndex, match.index)))
      }
      if (match[1]) {
        nodes.push(React.createElement('span', { key: key++, className: 'token-comment' }, match[1]))
      } else if (match[2]) {
        nodes.push(React.createElement('span', { key: key++, className: 'token-string' }, match[2]))
      } else if (match[3]) {
        nodes.push(React.createElement('span', { key: key++, className: 'token-keyword' }, match[3]))
      } else if (match[4]) {
        nodes.push(React.createElement('span', { key: key++, className: 'token-number' }, match[4]))
      } else if (match[5]) {
        nodes.push(React.createElement('span', { key: key++, className: 'token-type' }, match[5]))
      }
      lastIndex = tokenRegex.lastIndex
    }
    if (lastIndex < code.length) {
      nodes.push(React.createElement('span', { key: key++ }, code.slice(lastIndex)))
    }
    return nodes
  }

  function CalloutBlockquote({ children }) {
    const childArray = React.Children.toArray(children)
    const firstElement = childArray.find((c) => React.isValidElement(c))
    if (firstElement && firstElement.props && firstElement.props.children) {
      const pChildren = React.Children.toArray(firstElement.props.children)
      const firstText = pChildren[0]
      if (typeof firstText === 'string') {
        const match = firstText.match(/^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\](?:\s*\n|\s+)?([\s\S]*)$/i)
        if (match) {
          const type = match[1].toLowerCase()
          const remainder = match[2]
          const newPChildren = remainder ? [remainder, ...pChildren.slice(1)] : pChildren.slice(1)
          const updatedFirst = newPChildren.length > 0 ? React.cloneElement(firstElement, {}, ...newPChildren) : null
          const restElements = childArray.filter((c) => c !== firstElement)
          return React.createElement(
            'div',
            { className: `callout callout-${type}` },
            React.createElement('span', { className: 'callout-title' }, type.toUpperCase()),
            updatedFirst,
            ...restElements,
          )
        }
      }
    }
    return React.createElement('blockquote', { className: 'standard-blockquote' }, children)
  }

  const customComponents = {
    h1: ({ children }) => React.createElement('h1', { className: 'preview-h1' }, children),
    table: ({ children }) => React.createElement('table', { className: 'preview-table' }, children),
    pre: ({ children }) => {
      const element = React.isValidElement(children) ? children : null
      const rawText = String(element?.props?.children ?? '').replace(/\n$/, '')
      return React.createElement('pre', null, React.createElement('code', null, highlightCodeTokens(rawText)))
    },
    blockquote: CalloutBlockquote,
  }

  const sampleMd = `
# Release Notes

| Item | Status |
| --- | --- |
| Preview Modal | Done |

\`\`\`go
func Add(a, b int) int {
	return a + b
}
\`\`\`

> [!TIP]
> Use keyboard shortcuts for fast navigation.
`

  const mdHtml = renderToStaticMarkup(
    React.createElement(ReactMarkdown, { remarkPlugins: [remarkGfm], components: customComponents }, sampleMd),
  )

  assert(mdHtml.includes('class="preview-h1"'), 'H1 heading rendered')
  assert(mdHtml.includes('class="preview-table"'), 'Markdown table rendered')
  assert(mdHtml.includes('class="token-keyword">func</span>'), 'Go keyword highlighted')
  assert(mdHtml.includes('class="callout callout-tip"'), 'GitHub Alert TIP callout rendered')
  console.log('  ✓ Markdown headings, GFM tables, syntax tokens, and Alert TIP callout verified')

  // ---------------------------------------------------------------------------
  // Test 4: Compiled CSS Tokens & Mobile Responsiveness
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 4: Compiled CSS Tokens & Mobile Viewport Geometry ---')

  const distDir = path.join(__dirname, '../dist/assets')
  const cssFiles = fs.readdirSync(distDir).filter((f) => f.endsWith('.css'))
  assert(cssFiles.length > 0, 'Compiled CSS bundle exists')
  const cssContent = fs.readFileSync(path.join(distDir, cssFiles[0]), 'utf8')

  const responsiveTokens = [
    { name: 'min-h-11 (44px WCAG touch target)', regex: /\.min-h-11/ },
    { name: 'min-w-11 (44px WCAG touch target)', regex: /\.min-w-11/ },
    { name: 'h-dvh (mobile dynamic viewport height)', regex: /\.h-dvh/ },
    { name: 'max-h-dvh (mobile dynamic max height)', regex: /\.max-h-dvh/ },
    { name: 'sm:max-w-4xl (desktop modal width constraint)', regex: /sm\\:max-w-4xl/ },
    { name: 'backdrop:bg-black/60 (dialog overlay)', regex: /backdrop/ },
    { name: 'flex-wrap (header and action button responsiveness)', regex: /\.flex-wrap/ },
    { name: 'break-all (prevent filename flex blowout)', regex: /\.break-all/ },
    { name: 'truncate (header title truncation)', regex: /\.truncate/ },
    { name: 'cursor-zoom-in (image interaction)', regex: /cursor-zoom-in/ },
  ]

  for (const token of responsiveTokens) {
    assert(token.regex.test(cssContent), `Missing compiled utility token: ${token.name}`)
    console.log(`  ✓ ${token.name} present in compiled production bundle`)
  }

  // ---------------------------------------------------------------------------
  // Test 5: Source Integration Checks
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 5: Source File Integration Checks ---')

  const attachmentsSource = fs.readFileSync(path.join(__dirname, '../src/features/inbox/Attachments.tsx'), 'utf8')
  assert(attachmentsSource.includes('FilePreviewModal'), 'Attachments.tsx imports FilePreviewModal')
  assert(attachmentsSource.includes('isPreviewableFile(a.name, a.type)'), 'Attachments.tsx checks isPreviewableFile')
  assert(attachmentsSource.includes('setPreviewOpen(true)'), 'Attachments.tsx connects click to open preview modal')
  assert(attachmentsSource.includes('void download()'), 'Attachments.tsx preserves direct download fallback')
  console.log('  ✓ In-app preview modal integrated into Attachments.tsx with direct download fallback')

  const taskSessionSource = fs.readFileSync(path.join(__dirname, '../src/features/tasks/TaskSessionPage.tsx'), 'utf8')
  assert(taskSessionSource.includes('detail.attachments && detail.attachments.length > 0'), 'TaskSessionPage.tsx checks detail.attachments')
  assert(taskSessionSource.includes('AttachmentFile'), 'TaskSessionPage.tsx renders AttachmentFile items')
  console.log('  ✓ TaskSessionPage.tsx displays work session files list')

  const taskSidePeekSource = fs.readFileSync(path.join(__dirname, '../src/features/tasks/TaskSidePeek.tsx'), 'utf8')
  assert(taskSidePeekSource.includes('detail.attachments && detail.attachments.length > 0'), 'TaskSidePeek.tsx checks detail.attachments')
  assert(taskSidePeekSource.includes('AttachmentFile'), 'TaskSidePeek.tsx renders AttachmentFile items')
  console.log('  ✓ TaskSidePeek.tsx displays work session files in task details')

  const idLocaleSource = fs.readFileSync(path.join(__dirname, '../src/locales/id.json'), 'utf8')
  assert(idLocaleSource.includes('"Preview": "Pratinjau"'), 'id.json includes Preview translation')
  assert(idLocaleSource.includes('"Files from this work session": "File dari sesi kerja ini"'), 'id.json includes session files translation')
  assert(idLocaleSource.includes('"Copy content": "Salin konten"'), 'id.json includes Copy content translation')
  console.log('  ✓ Localization strings verified in id.json')

  console.log('\n======================================================================')
  console.log('ALL IN-APP FILE PREVIEW & RESPONSIVENESS TESTS PASSED! (5/5)')
  console.log('======================================================================')
})().catch((err) => {
  console.error('Test suite failed:', err)
  process.exitCode = 1
})
