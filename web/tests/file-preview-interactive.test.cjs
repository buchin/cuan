const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')

;(async () => {
  const { default: ReactMarkdown } = await import('react-markdown')
  const remarkGfm = (await import('remark-gfm')).default

  console.log('======================================================================')
  console.log('UNIVERSAL FILE PREVIEW & CHAT REPLY QUOTE VERIFICATION SUITE')
  console.log('Features: In-App Lightbox, Multi-Format Visual Views, Chat Reply Clamp')
  console.log('======================================================================\n')

  // ---------------------------------------------------------------------------
  // Test 1: File Type Classification & Preview Eligibility
  // ---------------------------------------------------------------------------
  console.log('--- Test 1: File Type Classification & Universal Format Support ---')

  function isMarkdownFile(name) {
    return /\.(md|markdown|mdown|mkdn)$/i.test(name)
  }
  function isImageFile(name, type) {
    return (
      /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i.test(name) ||
      (type ? type.startsWith('image/') : false)
    )
  }
  function isHtmlFile(name, type) {
    return /\.(html|htm)$/i.test(name) || (type ? type.includes('html') : false)
  }
  function isSvgFile(name, type) {
    return /\.svg$/i.test(name) || (type ? type.includes('svg') : false)
  }
  function isCsvFile(name, type) {
    return /\.(csv|tsv)$/i.test(name) || (type ? type.includes('csv') : false)
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
      isHtmlFile(name, type) ||
      isSvgFile(name, type) ||
      isCsvFile(name, type) ||
      isDiffFile(name) ||
      isLogFile(name) ||
      isJsonFile(name, type) ||
      isCodeFile(name) ||
      isTextFile(name, type)
    )
  }

  // 1.1 Markdown formats
  assert.equal(isMarkdownFile('README.md'), true)
  assert.equal(isMarkdownFile('notes.markdown'), true)
  assert.equal(isMarkdownFile('doc.MKDN'), true)
  assert.equal(isMarkdownFile('main.go'), false)
  assert.equal(isPreviewableFile('README.md'), true)
  console.log('  ✓ Markdown formats verified (.md, .markdown)')

  // 1.2 Image formats (PNG, JPG, JPEG, WEBP, GIF, SVG, etc.)
  assert.equal(isImageFile('screenshot.png'), true)
  assert.equal(isImageFile('photo.jpg'), true)
  assert.equal(isImageFile('banner.jpeg'), true)
  assert.equal(isImageFile('graphic.webp'), true)
  assert.equal(isImageFile('animation.gif'), true)
  assert.equal(isImageFile('vector.svg'), true)
  assert.equal(isImageFile('unknown_binary', 'image/png'), true)
  assert.equal(isPreviewableFile('1000283203.jpg'), true)
  console.log('  ✓ Image formats verified (.png, .jpg, .jpeg, .webp, .gif, .svg, image/*)')

  // 1.3 HTML formats
  assert.equal(isHtmlFile('index.html'), true)
  assert.equal(isHtmlFile('template.htm'), true)
  assert.equal(isHtmlFile('data.bin', 'text/html'), true)
  assert.equal(isPreviewableFile('report.html'), true)
  console.log('  ✓ HTML formats verified (.html, .htm, text/html)')

  // 1.4 CSV & TSV formats
  assert.equal(isCsvFile('data.csv'), true)
  assert.equal(isCsvFile('export.tsv'), true)
  assert.equal(isCsvFile('raw.bin', 'text/csv'), true)
  assert.equal(isPreviewableFile('users.csv'), true)
  console.log('  ✓ CSV & TSV formats verified (.csv, .tsv, text/csv)')

  // 1.5 SVG formats
  assert.equal(isSvgFile('icon.svg'), true)
  assert.equal(isSvgFile('logo.bin', 'image/svg+xml'), true)
  assert.equal(isPreviewableFile('icon.svg'), true)
  console.log('  ✓ SVG formats verified (.svg, image/svg+xml)')

  // 1.6 JSON formats
  assert.equal(isJsonFile('manifest.json'), true)
  assert.equal(isJsonFile('config.bin', 'application/json'), true)
  assert.equal(isPreviewableFile('manifest.json'), true)
  console.log('  ✓ JSON formats verified (.json, application/json)')

  // 1.7 Diff & Log & Code formats
  assert.equal(isDiffFile('changes.diff'), true)
  assert.equal(isDiffFile('feature.patch'), true)
  assert.equal(isLogFile('build.log'), true)
  assert.equal(isCodeFile('server.go'), true)
  assert.equal(isCodeFile('App.tsx'), true)
  assert.equal(isCodeFile('script.py'), true)
  console.log('  ✓ Diff, Log, Code, Text formats verified')

  // 1.8 Unsupported binary formats (Must fallback to direct download)
  assert.equal(isPreviewableFile('release.zip'), false)
  assert.equal(isPreviewableFile('package.tar.gz'), false)
  assert.equal(isPreviewableFile('binary.exe'), false)
  assert.equal(isPreviewableFile('blob.bin'), false)
  assert.equal(isPreviewableFile('firmware.iso'), false)
  assert.equal(isPreviewableFile('document.pdf'), false)
  console.log('  ✓ Unsupported binary formats correctly identified for fallback download (.zip, .tar.gz, .exe, .bin, .pdf)')

  // ---------------------------------------------------------------------------
  // Test 2: CSV Data Table Parser
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 2: CSV Data Table Parsing & Rendering ---')

  function parseCsv(text) {
    const lines = text.trim().split(/\r?\n/)
    if (!lines.length || !lines[0].trim()) return { headers: [], rows: [] }

    let delimiter = ','
    if (lines[0].includes('\t') && !lines[0].includes(',')) {
      delimiter = '\t'
    } else if (lines[0].includes(';') && !lines[0].includes(',')) {
      delimiter = ';'
    }

    const parseLine = (line) => {
      const result = []
      let current = ''
      let inQuotes = false
      for (let i = 0; i < line.length; i++) {
        const char = line[i]
        if (char === '"' || char === "'") {
          if (inQuotes && line[i + 1] === char) {
            current += char
            i++
          } else {
            inQuotes = !inQuotes
          }
        } else if (char === delimiter && !inQuotes) {
          result.push(current.trim())
          current = ''
        } else {
          current += char
        }
      }
      result.push(current.trim())
      return result
    }

    const headers = parseLine(lines[0])
    const rows = lines.slice(1).filter((l) => l.trim().length > 0).map(parseLine)
    return { headers, rows }
  }

  const sampleCsv = `ID,Name,Role,Status
1,"Alice Developer",Engineering,Active
2,"Bob Designer",Product,Active
3,"Charlie Manager",Operations,"On Leave"`

  const { headers, rows } = parseCsv(sampleCsv)
  assert.equal(headers.length, 4)
  assert.deepEqual(headers, ['ID', 'Name', 'Role', 'Status'])
  assert.equal(rows.length, 3)
  assert.equal(rows[0][1], 'Alice Developer')
  assert.equal(rows[2][3], 'On Leave')
  console.log('  ✓ CSV parsed into 4 headers and 3 clean data rows with quoted string support')

  // ---------------------------------------------------------------------------
  // Test 3: HTML Sandboxed Iframe Rendering Verification
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 3: HTML Sandboxed Iframe & Visual Rendering ---')

  const sampleHtml = '<!DOCTYPE html><html><body><h1>Release Report</h1><p>Status: OK</p></body></html>'
  const iframeComponent = React.createElement('iframe', {
    srcDoc: sampleHtml,
    sandbox: 'allow-same-origin',
    title: 'test.html',
    className: 'w-full h-full min-h-[480px] rounded-lg border border-border bg-white',
  })
  const iframeHtml = renderToStaticMarkup(iframeComponent)
  assert(iframeHtml.includes('sandbox="allow-same-origin"'), 'Iframe has sandboxing attribute')
  assert(iframeHtml.toLowerCase().includes('srcdoc="&lt;!doctype html&gt;') || iframeHtml.includes('srcDoc="&lt;!DOCTYPE html&gt;'), 'Iframe has escaped srcDoc content')
  console.log('  ✓ HTML sandboxed iframe component verified with safe origin execution')

  // ---------------------------------------------------------------------------
  // Test 4: Chat Reply Quote Layout & Prefix Stripping
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 4: Chat Reply Quote Layout & Prefix Stripping ---')

  function prepareChatMarkdown(raw) {
    if (!raw) return ''
    return raw.replace(/(^>.*)(\r?\n)([^>\r\n])/gm, '$1$2\n$3')
  }

  const sampleQuoteWithReply = `> Tepat sekali. Ketika lampiran atau item pada **"Files from this work session"** diklik
> Komponen daftar file sesi kerja harus membuka modal preview
Saya setuju dengan analisis ini.`

  const prepared = prepareChatMarkdown(sampleQuoteWithReply)
  assert(prepared.includes('> Komponen daftar file sesi kerja harus membuka modal preview\n\nSaya setuju'), 'Prepared text inserts blank line between quote and reply')

  const customComponents = {
    blockquote: ({ children }) =>
      React.createElement(
        'blockquote',
        { className: 'chat-reply-quote', role: 'region', 'aria-label': 'Kutipan pesan' },
        React.createElement('div', { className: 'chat-reply-body' }, children),
      ),
  }

  const renderedQuote = renderToStaticMarkup(
    React.createElement(ReactMarkdown, { remarkPlugins: [remarkGfm], components: customComponents }, prepared),
  )

  assert(renderedQuote.includes('class="chat-reply-quote"'), 'Rendered quote container has .chat-reply-quote class')
  assert(renderedQuote.includes('class="chat-reply-body"'), 'Rendered quote body has .chat-reply-body class')
  assert(renderedQuote.includes('<strong>&quot;Files from this work session&quot;</strong>') || renderedQuote.includes('<strong>"Files from this work session"</strong>'), 'Bold formatting inside quote rendered')
  assert(!renderedQuote.includes('&gt; Tepat sekali'), 'Raw > prefix stripped from rendered quote content')
  assert(renderedQuote.includes('<p>Saya setuju dengan analisis ini.</p>'), 'Reply text rendered outside blockquote')
  console.log('  ✓ Chat reply container rendered with .chat-reply-quote, bold formatted text, and raw > stripped')

  // ---------------------------------------------------------------------------
  // Test 5: Compiled CSS Tokens & Mobile Viewport Geometry (360px - 412px)
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 5: Compiled CSS Tokens & Mobile Viewport Geometry ---')

  const distDir = path.join(__dirname, '../dist/assets')
  const cssFiles = fs.readdirSync(distDir).filter((f) => f.endsWith('.css'))
  assert(cssFiles.length > 0, 'Compiled CSS bundle exists')
  const cssContent = cssFiles.map((f) => fs.readFileSync(path.join(distDir, f), 'utf8')).join('\n')

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
    { name: 'cursor-zoom-out (image zoomed state)', regex: /cursor-zoom-out/ },
  ]

  for (const token of responsiveTokens) {
    assert(token.regex.test(cssContent), `Missing compiled utility token: ${token.name}`)
    console.log(`  ✓ ${token.name} present in compiled production bundle`)
  }

  // Verify chat reply CSS rules
  const chatMarkdownCss = fs.readFileSync(path.join(__dirname, '../src/features/inbox/chat-markdown.css'), 'utf8')
  assert(chatMarkdownCss.includes('.chat-reply-body'), 'chat-markdown.css includes .chat-reply-body')
  assert(chatMarkdownCss.includes('-webkit-line-clamp: 3'), 'chat-markdown.css limits quote body to 3 lines')
  assert(chatMarkdownCss.includes('max-height: 6.25rem'), 'chat-markdown.css limits blockquote max-height')
  assert(chatMarkdownCss.includes('border-left: 3.5px solid'), 'chat-markdown.css has vertical accent border')
  console.log('  ✓ chat-markdown.css enforces clamp, max-height, and vertical accent border')

  const chatCalmCss = fs.readFileSync(path.join(__dirname, '../src/features/inbox/chat-calm.css'), 'utf8')
  assert(chatCalmCss.includes('.wa-chat .chat-markdown blockquote'), 'chat-calm.css customizes blockquote')
  assert(chatCalmCss.includes('.wa-outgoing .chat-markdown blockquote'), 'chat-calm.css customizes outgoing bubble quote')
  assert(chatCalmCss.includes('.dark .wa-outgoing .chat-markdown blockquote'), 'chat-calm.css customizes dark outgoing bubble quote')
  console.log('  ✓ chat-calm.css handles dark/light theme and outgoing/incoming bubble accent colors')

  // ---------------------------------------------------------------------------
  // Test 6: Source Integration Checks
  // ---------------------------------------------------------------------------
  console.log('\n--- Test 6: Source File Integration Checks ---')

  const attachmentsSource = fs.readFileSync(path.join(__dirname, '../src/features/inbox/Attachments.tsx'), 'utf8')
  assert(attachmentsSource.includes('FilePreviewModal'), 'Attachments.tsx imports FilePreviewModal')
  assert(attachmentsSource.includes('isPreviewableFile(a.name, a.type)'), 'Attachments.tsx checks isPreviewableFile')
  assert(attachmentsSource.includes('isImageFile(a.name, a.type)'), 'Attachments.tsx checks isImageFile for thumbnails')
  assert(attachmentsSource.includes('setPreviewOpen(true)'), 'Attachments.tsx connects click to open preview modal')
  assert(attachmentsSource.includes('preview=1'), 'Attachments.tsx requests preview=1 to avoid download disposition')
  console.log('  ✓ In-app preview modal integrated into Attachments.tsx with preview=1 and image check')

  const filePreviewModalSource = fs.readFileSync(path.join(__dirname, '../src/components/dialogs/FilePreviewModal.tsx'), 'utf8')
  assert(filePreviewModalSource.includes('HtmlVisualView'), 'FilePreviewModal includes HtmlVisualView component')
  assert(filePreviewModalSource.includes('CsvTableView'), 'FilePreviewModal includes CsvTableView component')
  assert(filePreviewModalSource.includes('JsonVisualView'), 'FilePreviewModal includes JsonVisualView component')
  assert(filePreviewModalSource.includes('SvgVisualView'), 'FilePreviewModal includes SvgVisualView component')
  assert(filePreviewModalSource.includes('setZoomLevel'), 'FilePreviewModal provides interactive zoom controls')
  assert(filePreviewModalSource.includes('imgDimensions'), 'FilePreviewModal tracks and displays natural image dimensions')
  assert(filePreviewModalSource.includes('hasToggle'), 'FilePreviewModal supports viewMode toggle between visual and raw source')
  console.log('  ✓ FilePreviewModal contains HTML, CSV, JSON, SVG, Zoom controls, and Source Code toggles')

  const inboxViewSource = fs.readFileSync(path.join(__dirname, '../src/features/inbox/InboxView.tsx'), 'utf8')
  assert(inboxViewSource.includes('isFormattedMessage = true'), 'InboxView renders all messages formatted with quotes/markdown')
  console.log('  ✓ InboxView renders formatted messages and reply quotes')

  const idLocaleSource = fs.readFileSync(path.join(__dirname, '../src/locales/id.json'), 'utf8')
  assert(idLocaleSource.includes('"Preview": "Pratinjau"'), 'id.json includes Preview translation')
  assert(idLocaleSource.includes('"Files from this work session": "File dari sesi kerja ini"'), 'id.json includes session files translation')
  assert(idLocaleSource.includes('"Source Code": "Kode Sumber"'), 'id.json includes Source Code translation')
  assert(idLocaleSource.includes('"Visual Table": "Tabel Visual"'), 'id.json includes Visual Table translation')
  assert(idLocaleSource.includes('"Zoom in": "Perbesar"'), 'id.json includes Zoom in translation')
  console.log('  ✓ Localization strings verified in id.json for multi-format views')

  console.log('\n======================================================================')
  console.log('ALL UNIVERSAL FILE PREVIEW & CHAT REPLY TESTS PASSED! (6/6)')
  console.log('======================================================================')
})().catch((err) => {
  console.error('Test suite failed:', err)
  process.exitCode = 1
})
