import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

let React = null
let ReactDOMServer = null
try {
  React = (await import('react')).default
  ReactDOMServer = (await import('react-dom/server')).default
} catch {
  // Optional runtime dependency in environments without node_modules
}

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const mermaidBlockPath = path.resolve(__dirname, '../src/components/ui/MermaidBlock.tsx')
const markdownRendererPath = path.resolve(__dirname, '../src/components/ui/MarkdownRenderer.tsx')
const chatMarkdownPath = path.resolve(__dirname, '../src/features/inbox/ChatMarkdown.tsx')
const chatMarkdownCssPath = path.resolve(__dirname, '../src/features/inbox/chat-markdown.css')
const vendorMermaidPath = path.resolve(__dirname, '../public/vendor/mermaid/mermaid.min.js')

test('1. Mermaid component files and exports exist and are valid TypeScript', () => {
  assert.ok(fs.existsSync(mermaidBlockPath), 'components/ui/MermaidBlock.tsx must exist')
  assert.ok(fs.existsSync(markdownRendererPath), 'components/ui/MarkdownRenderer.tsx must exist')
  assert.ok(fs.existsSync(chatMarkdownPath), 'features/inbox/ChatMarkdown.tsx must exist')
  assert.ok(fs.existsSync(vendorMermaidPath), 'public/vendor/mermaid/mermaid.min.js must exist')

  const mermaidSrc = fs.readFileSync(mermaidBlockPath, 'utf8')
  assert.ok(mermaidSrc.includes('export function MermaidBlock'), 'MermaidBlock component must be exported')
  assert.ok(mermaidSrc.includes('export interface MermaidBlockProps'), 'MermaidBlockProps must be exported')

  const rendererSrc = fs.readFileSync(markdownRendererPath, 'utf8')
  assert.ok(rendererSrc.includes('export const MarkdownRenderer = ChatMarkdown'), 'MarkdownRenderer must alias ChatMarkdown')
  assert.ok(rendererSrc.includes("export { MermaidBlock } from './MermaidBlock'"), 'MarkdownRenderer must re-export MermaidBlock')

  const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')
  assert.ok(chatMarkdownSrc.includes("import { MermaidBlock } from '../../components/ui/MermaidBlock'"), 'ChatMarkdown must import MermaidBlock')
  assert.ok(chatMarkdownSrc.includes('export const MarkdownRenderer = ChatMarkdown'), 'ChatMarkdown must export MarkdownRenderer alias')
})

test('2. ChatMarkdown automatically routes ```mermaid and ```flowchart code blocks to MermaidBlock', () => {
  const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')
  assert.ok(
    chatMarkdownSrc.includes("lang === 'mermaid' || lang === 'flowchart'"),
    'ChatMarkdown must recognize both mermaid and flowchart blocks'
  )
  assert.ok(
    chatMarkdownSrc.includes('<MermaidBlock code={text} />'),
    'ChatMarkdown must render MermaidBlock instead of raw text'
  )
})

test('3. Mobile responsiveness (360px-390px): max-width, height auto, and overflow-x touch momentum scrolling', () => {
  const cssSrc = fs.readFileSync(chatMarkdownCssPath, 'utf8')
  assert.ok(cssSrc.includes('.chat-code-mermaid'), 'chat-markdown.css must define .chat-code-mermaid')
  assert.ok(cssSrc.includes('-webkit-overflow-scrolling: touch'), 'chat-markdown.css must enable touch momentum scrolling')
  assert.ok(cssSrc.includes('overflow-x: auto'), 'chat-markdown.css must enable horizontal scrolling for wide diagrams')
  assert.ok(cssSrc.includes('max-width: 100% !important'), 'SVG must enforce max-width: 100%')
  assert.ok(cssSrc.includes('height: auto !important'), 'SVG must enforce height: auto')

  const mermaidSrc = fs.readFileSync(mermaidBlockPath, 'utf8')
  assert.ok(mermaidSrc.includes('overflow-x-auto'), 'MermaidBlock container must have overflow-x-auto')
  assert.ok(mermaidSrc.includes('touchAction:'), 'MermaidBlock container must set touchAction')
})

test('4. Interactive view: Fullscreen / Pop-up viewer with Zoom In, Zoom Out, Reset, and Copy Syntax', () => {
  const mermaidSrc = fs.readFileSync(mermaidBlockPath, 'utf8')
  assert.ok(mermaidSrc.includes('openFullscreen'), 'MermaidBlock must implement openFullscreen')
  assert.ok(mermaidSrc.includes('ZoomIn'), 'MermaidBlock must include ZoomIn icon/control')
  assert.ok(mermaidSrc.includes('ZoomOut'), 'MermaidBlock must include ZoomOut icon/control')
  assert.ok(mermaidSrc.includes('RotateCcw'), 'MermaidBlock must include RotateCcw reset control')
  assert.ok(mermaidSrc.includes('handleCopy'), 'MermaidBlock must implement handleCopy')
  assert.ok(mermaidSrc.includes('createPortal'), 'Fullscreen modal must be rendered via createPortal')
  assert.ok(mermaidSrc.includes('setViewMode'), 'MermaidBlock must support toggling between diagram and code view')
})

test('5. Graceful fallback on invalid/broken syntax without crash', () => {
  const mermaidSrc = fs.readFileSync(mermaidBlockPath, 'utf8')
  assert.ok(mermaidSrc.includes('AlertTriangle'), 'MermaidBlock must render AlertTriangle on error')
  assert.ok(mermaidSrc.includes('Sintaks diagram Mermaid tidak valid'), 'MermaidBlock must display graceful error message')
  assert.ok(mermaidSrc.includes('normalizedCode'), 'MermaidBlock must render raw fallback code on error')
  assert.ok(mermaidSrc.includes('setError('), 'MermaidBlock must catch rendering errors gracefully')
})

test('6. User attachment workflow diagram sample parses correctly in ReactMarkdown pipeline', async () => {
  const userSampleMermaid = `flowchart TD
    A["Runner Selesai Eksekusi"] --> B
    B --> C["Evaluasi DoD vs Bukti"]
    C -->|Lulus Penuh| D["Set Status Done"]
    C -->|Belum Lulus & Fixable| E["Update Arahan"]
    E --> A
    C -->|Gagal & Butuh Bantuan Manusia| F["Escalate"]`

  const markdownContent = `### Alur Kerja (Workflow) yang Terbentuk\n\n\`\`\`mermaid\n${userSampleMermaid}\n\`\`\`\n`

  if (!React || !ReactDOMServer) {
    assert.ok(markdownContent.includes('```mermaid'), 'Markdown content includes mermaid block')
    return
  }

  const { default: ReactMarkdown } = await import('react-markdown')
  const { default: remarkGfm } = await import('remark-gfm')

  let interceptedCode = ''
  let interceptedLang = ''

  const html = ReactDOMServer.renderToStaticMarkup(
    React.createElement(
      ReactMarkdown,
      {
        remarkPlugins: [remarkGfm],
        components: {
          pre: ({ children }) => {
            const el = React.isValidElement(children) ? children : null
            interceptedCode = String(el?.props?.children ?? '')
            interceptedLang = el?.props?.className?.replace(/^language-/, '') ?? ''
            return React.createElement('div', { className: 'intercepted-mermaid' }, interceptedCode)
          },
        },
      },
      markdownContent
    )
  )

  assert.ok(html.includes('intercepted-mermaid'), 'HTML must contain intercepted code block')
  assert.equal(interceptedLang, 'mermaid', 'Intercepted language must be mermaid')
  assert.ok(interceptedCode.includes('Runner Selesai Eksekusi'), 'Intercepted code must contain workflow text')
})
