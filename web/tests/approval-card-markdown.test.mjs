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
const approvalCardPath = path.resolve(__dirname, '../src/features/inbox/ApprovalCard.tsx')
const approvalCardUiPath = path.resolve(__dirname, '../src/components/ui/ApprovalCard.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatMarkdownCssPath = path.resolve(__dirname, '../src/features/inbox/chat-markdown.css')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')

// Dynamically load compiled production CSS
const distAssetsDir = path.resolve(__dirname, '../dist/assets')
let compiledCss = ''
if (fs.existsSync(distAssetsDir)) {
  const cssFile = fs.readdirSync(distAssetsDir).find((f) => f.endsWith('.css'))
  if (cssFile) {
    compiledCss = fs.readFileSync(path.join(distAssetsDir, cssFile), 'utf8')
  }
}

test('1. ApprovalCard component files and TypeScript interfaces exist', () => {
  assert.ok(fs.existsSync(approvalCardPath), 'features/inbox/ApprovalCard.tsx must exist')
  assert.ok(fs.existsSync(approvalCardUiPath), 'components/ui/ApprovalCard.tsx re-export must exist')

  const cardSrc = fs.readFileSync(approvalCardPath, 'utf8')
  assert.ok(cardSrc.includes('export function ApprovalCard'), 'ApprovalCard component must be exported')
  assert.ok(cardSrc.includes('export function FormattedTitle'), 'FormattedTitle helper must be exported')
  assert.ok(cardSrc.includes('export interface ApprovalCardProps'), 'ApprovalCardProps interface must be exported')

  const chatApiSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(chatApiSrc.includes('export interface ApprovalPlan'), 'api/chat.ts must export ApprovalPlan')
  assert.ok(chatApiSrc.includes('approval_plan?: ApprovalPlan'), 'ChatMessage must use typed approval_plan')
})

test('2. ApprovalCard uses ChatMarkdown for plan description and handles Markdown elements', async () => {
  const cardSrc = fs.readFileSync(approvalCardPath, 'utf8')
  assert.ok(
    cardSrc.includes("import { ChatMarkdown } from './ChatMarkdown'"),
    'ApprovalCard must import ChatMarkdown'
  )
  assert.ok(
    cardSrc.includes('<ChatMarkdown content={plan.description}'),
    'ApprovalCard must render plan.description using ChatMarkdown'
  )

  // Verify Markdown parser capability on the actual sample data from user attachment 1000283721.jpg
  if (!React || !ReactDOMServer) {
    const chatMarkdownSrc = fs.readFileSync(path.resolve(__dirname, '../src/features/inbox/ChatMarkdown.tsx'), 'utf8')
    assert.ok(chatMarkdownSrc.includes('react-markdown'), 'ChatMarkdown must import react-markdown')
    assert.ok(chatMarkdownSrc.includes('remark-gfm'), 'ChatMarkdown must import remark-gfm')
    return
  }

  const { default: ReactMarkdown } = await import('react-markdown')
  const { default: remarkGfm } = await import('remark-gfm')

  const sampleDescription = `Berdasarkan tangkapan layar yang Anda kirimkan, ada dua persoalan utama pada tampilan mobile:

1. **Toolbar Virtual Key / Keyboard Tambahan (\`Ctrl+C\`, \`Enter\`, \`ESC\`, \`TAB\`, panah, dll.)**:
- Saat ini toolbar keyboard virtual dan tombol terminal selalu menempel di bagian bawah layar. Di layar mobile, baris tombol ini memakan hampir **40-50% area vertikal** viewport ketika virtual keyboard OS terbuka, sehingga ruang baca konten terminal/log menjadi sangat sempit (hanya tersisa beberapa baris teks).
- **Evaluasi**: **Sangat bagus dan tepat jika keyboard tambahan ini hanya muncul...

\`\`\`bash
echo "Verifikasi konfigurasi mobile keyboard"
\`\`\`
| Fitur | Status |
| --- | --- |
| Markdown | Didukung |`;

  const renderedHtml = ReactDOMServer.renderToStaticMarkup(
    React.createElement(ReactMarkdown, { remarkPlugins: [remarkGfm] }, sampleDescription)
  )

  // 1. Numbered lists (<ol><li>)
  assert.ok(renderedHtml.includes('<ol>') && renderedHtml.includes('<li>'), 'Markdown must render numbered lists (<ol><li>)')
  // 2. Unordered bullet lists (<ul><li>)
  assert.ok(renderedHtml.includes('<ul>') && renderedHtml.includes('<li>'), 'Markdown must render bullet points (<ul><li>)')
  // 3. Bold text styling (<strong>)
  assert.ok(renderedHtml.includes('<strong>40-50% area vertikal</strong>'), 'Markdown must render bold text (<strong>)')
  // 4. Inline code (<code>)
  assert.ok(renderedHtml.includes('<code>Ctrl+C</code>'), 'Markdown must render inline code with backticks (<code>)')
  // 5. Code block (<pre><code class="language-bash">)
  assert.ok(renderedHtml.includes('<pre><code class="language-bash">'), 'Markdown must render fenced code blocks (<pre><code>)')
  // 6. Table (<table>)
  assert.ok(renderedHtml.includes('<table>') && renderedHtml.includes('<th>Fitur</th>'), 'Markdown must render tables')
})

test('3. FormattedTitle parses and formats backticked inline code within titles', () => {
  const cardSrc = fs.readFileSync(approvalCardPath, 'utf8')
  assert.ok(
    cardSrc.includes("title.split(/(`[^`]+`)/g)"),
    'FormattedTitle must parse backtick code segments'
  )
  assert.ok(
    cardSrc.includes('<code'),
    'FormattedTitle must render code tag for backticked title tokens'
  )
  assert.ok(
    cardSrc.includes('font-mono'),
    'FormattedTitle code tags must use font-mono'
  )
})

test('4. ApprovalCard supports Pending, Accepted, and Deferred visual states and actions', () => {
  const cardSrc = fs.readFileSync(approvalCardPath, 'utf8')

  // Status tokens
  assert.ok(cardSrc.includes("data-status={accepted ? 'accepted' : deferred ? 'deferred' : 'pending'}"), 'Card must declare data-status attribute')
  assert.ok(cardSrc.includes('border-l-primary'), 'Pending state must use primary left border accent')
  assert.ok(cardSrc.includes('border-l-emerald-500'), 'Accepted state must use emerald left border accent')
  assert.ok(cardSrc.includes('border-l-muted-foreground'), 'Deferred state must use muted left border accent')

  // Icons
  assert.ok(cardSrc.includes('ClipboardCheck'), 'Must use ClipboardCheck for pending state')
  assert.ok(cardSrc.includes('CheckCircle2') || cardSrc.includes('CheckCheck'), 'Must use Check icon for accepted state')
  assert.ok(cardSrc.includes('Clock3'), 'Must use Clock3 for deferred state')

  // Action buttons
  assert.ok(cardSrc.includes("t(accepted ? 'Approved' : 'Approve and start')"), 'Approve button text must be dynamic')
  assert.ok(cardSrc.includes("t('Change plan')"), 'Must render Change plan action')
  assert.ok(cardSrc.includes("t(deferred ? 'Saved for later' : 'Not now')"), 'Must render Not now / Saved for later action')
})

test('5. Mobile responsiveness & touch accessibility (WCAG 44px min-h-11 & anti-overflow)', () => {
  const cardSrc = fs.readFileSync(approvalCardPath, 'utf8')

  // Touch target height min-h-11 (44px)
  assert.ok(cardSrc.includes('min-h-11'), 'Action buttons must specify min-h-11 for 44px mobile touch targets')

  // Anti-overflow containers
  assert.ok(cardSrc.includes('min-w-0'), 'ApprovalCard must have min-w-0 to prevent flex/grid blowouts')
  assert.ok(cardSrc.includes('max-w-full'), 'ApprovalCard must have max-w-full to prevent horizontal viewport scrolling')
  assert.ok(cardSrc.includes('overflow-hidden'), 'ApprovalCard must have overflow-hidden to contain wide pre/tables')
  assert.ok(cardSrc.includes('flex-wrap'), 'Action buttons must wrap on small mobile viewports (flex-wrap)')
})

test('6. Scoped CSS in chat-markdown.css and compiled production CSS', () => {
  const cssSrc = fs.readFileSync(chatMarkdownCssPath, 'utf8')
  assert.ok(cssSrc.includes('.approval-card .chat-markdown'), 'chat-markdown.css must define .approval-card .chat-markdown rules')
  assert.ok(cssSrc.includes('.approval-card .chat-markdown ul'), 'chat-markdown.css must define list styles for approval-card')
  assert.ok(cssSrc.includes('.approval-card .chat-markdown pre'), 'chat-markdown.css must define pre/code block styles for approval-card')

  if (compiledCss) {
    assert.ok(compiledCss.includes('.approval-card'), 'Production CSS must include .approval-card rules')
    assert.ok(compiledCss.includes('min-h-11'), 'Production CSS must include min-h-11 touch target utility')
  }
})

test('7. InboxView integrates ApprovalCard and avoids duplicate headers', () => {
  const inboxSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxSrc.includes("import { ApprovalCard } from './ApprovalCard'"),
    'InboxView must import ApprovalCard'
  )
  assert.ok(
    inboxSrc.includes('<ApprovalCard'),
    'InboxView must render ApprovalCard'
  )
  assert.ok(
    inboxSrc.includes('msg.approval_title && !msg.approval_plan'),
    'InboxView must suppress redundant approval_title header when approval_plan exists'
  )
})
