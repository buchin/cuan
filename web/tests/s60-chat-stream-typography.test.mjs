import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const chatCalmCssPath = path.join(webDir, 'src/features/inbox/chat-calm.css')
const approvalCardPath = path.join(webDir, 'src/features/inbox/ApprovalCard.tsx')
const taskCompletionCardPath = path.join(webDir, 'src/features/inbox/TaskCompletionCard.tsx')
const taskCardPath = path.join(webDir, 'src/features/inbox/TaskCard.tsx')
const statusReportCardPath = path.join(webDir, 'src/components/ui/StatusReportCard.tsx')
const tasteChipsPath = path.join(webDir, 'src/features/inbox/TasteGroundingChips.tsx')
const inboxViewPath = path.join(webDir, 'src/features/inbox/InboxView.tsx')

test('1. S60-T4: Chat Stream Typography & Geist Variable in chat-calm.css', () => {
  assert.ok(fs.existsSync(chatCalmCssPath), 'chat-calm.css must exist')
  const css = fs.readFileSync(chatCalmCssPath, 'utf8')

  // Geist Variable font specification
  assert.ok(
    css.includes("font-family: 'Geist Variable'"),
    'chat-calm.css must specify Geist Variable font'
  )

  // Relaxed typographic leading (1.62) and subtle negative tracking
  assert.ok(
    css.includes('line-height: 1.62'),
    'chat-calm.css must provide relaxed line-height (1.62) for comfortable reading'
  )
  assert.ok(
    css.includes('letter-spacing: -0.011em'),
    'chat-calm.css must specify refined letter-spacing (-0.011em)'
  )

  // Message bubbles have hairline subtle borders and no heavy drop shadow
  assert.ok(
    css.includes('box-shadow: none'),
    'chat-calm.css message bubbles must eliminate heavy drop-shadows'
  )
  assert.ok(
    css.includes('border: 1px solid oklch(var(--border) / 0.2)'),
    'chat-calm.css message bubbles must use subtle hairline borders'
  )
})

test('2. S60-T4: Seamless Borderless Artifacts in chat-calm.css and InboxView.tsx', () => {
  const css = fs.readFileSync(chatCalmCssPath, 'utf8')
  const inboxView = fs.readFileSync(inboxViewPath, 'utf8')

  // Mixed text/card messages keep their normal message boundary. Browser
  // evidence checks computed background and padding with a real task card.
  assert.ok(css.includes('background: var(--wa-incoming)'), 'Incoming messages need their soft surface')
  assert.ok(!css.includes('.wa-message-bubble:has([data-card="task-card"])'), 'Descendant task cards must not erase an entire message bubble')
  assert.ok(css.includes('padding: 8px 14px 8px'), 'Messages retain readable padding')
  // Standalone completed tasks in InboxView eliminates duplicate borders
  assert.ok(
    inboxView.includes('className="inbox-attachment relative"'),
    'Standalone task previews use a plain attachment frame without a second speech surface'
  )
})

test('3. S60-T4: ApprovalCard calm typography & elimination of thick green card', () => {
  assert.ok(fs.existsSync(approvalCardPath), 'ApprovalCard.tsx must exist')
  const src = fs.readFileSync(approvalCardPath, 'utf8')

  // Elimination of heavy border-l-4 in favor of hairline border-l-2
  assert.ok(
    !src.includes('border-l-4'),
    'ApprovalCard must eliminate thick border-l-4 borders'
  )
  assert.ok(
    src.includes('border-l-2 border-l-emerald-500'),
    'ApprovalCard accepted state must use calm border-l-2 border-l-emerald-500'
  )

  // Elimination of loud green saturated background
  assert.ok(
    !src.includes('bg-emerald-500/[0.04]') && !src.includes('bg-emerald-500/[0.08]'),
    'ApprovalCard must eliminate heavy emerald-500 saturated background'
  )

  // Calm inline indicator ✓ Disetujui / ✓ Approved
  assert.ok(
    src.includes("{accepted ? '✓ ' : ''}"),
    'ApprovalCard must prepend gentle checkmark ✓ for accepted status'
  )

  // Action button when accepted is borderless / non-neon
  assert.ok(
    src.includes('bg-transparent hover:bg-transparent text-emerald-600'),
    'ApprovalCard accepted action button must be transparent, non-neon inline text'
  )
})

test('4. S60-T4: TaskCompletionCard Card Diet & Total Elimination of action-primary', () => {
  assert.ok(fs.existsSync(taskCompletionCardPath), 'TaskCompletionCard.tsx must exist')
  const src = fs.readFileSync(taskCompletionCardPath, 'utf8')

  // Total elimination of action-primary neon blue button
  assert.ok(
    !src.includes('action-primary'),
    'TaskCompletionCard must completely eliminate action-primary neon buttons'
  )

  // Replaced with subtle inline link and chevron
  assert.ok(
    src.includes('data-testid="completion-card-action-btn"'),
    'Must maintain data-testid="completion-card-action-btn"'
  )
  assert.ok(
    src.includes('text-muted-foreground hover:text-foreground'),
    'Action button must use calm text-muted-foreground hover:text-foreground'
  )
  assert.ok(
    src.includes('ChevronRight'),
    'Action button must render elegant ChevronRight'
  )

  // Elimination of nested box on summary
  const summaryBlock = src.substring(
    src.indexOf('data-testid="completion-card-summary"'),
    src.indexOf('prose prose-xs')
  )
  assert.ok(
    !summaryBlock.includes('border-border/80') && !summaryBlock.includes('rounded-xl bg-card border'),
    'Summary section must eliminate heavy nested card borders'
  )

  // DoD checklist formatted as relaxed, breathing typographic list (no black box)
  assert.ok(
    !src.includes('rounded-xl bg-background/60 border border-border/60'),
    'DoD checklist must eliminate the heavy black boxed container'
  )
  assert.ok(
    src.includes("t('Kriteria Selesai (DoD)')"),
    'Must preserve localized DoD section header'
  )
})

test('5. S60-T4: TaskCard elimination of action-primary & calm hairline aesthetics', () => {
  assert.ok(fs.existsSync(taskCardPath), 'TaskCard.tsx must exist')
  const src = fs.readFileSync(taskCardPath, 'utf8')

  // Total elimination of action-primary neon blue buttons in TaskCard
  assert.ok(
    !src.includes('action-primary'),
    'TaskCard must completely eliminate action-primary buttons'
  )

  // Preserves WCAG 44x44px touch targets with calm inline styling
  assert.ok(
    src.includes('min-h-[44px] min-w-[44px]'),
    'TaskCard must maintain WCAG 44x44px touch targets'
  )
  assert.ok(
    src.includes('text-muted-foreground hover:text-foreground'),
    'Action buttons must use subtle text-muted-foreground styling'
  )

  // Calm hairline container
  assert.ok(
    src.includes('border-border/40 bg-card/40'),
    'TaskCard container must use calm hairline border-border/40 and bg-card/40'
  )
})

test('6. S60-T4: StatusReportCard & TasteGroundingChips Clutter Elimination', () => {
  assert.ok(fs.existsSync(statusReportCardPath), 'StatusReportCard.tsx must exist')
  assert.ok(fs.existsSync(tasteChipsPath), 'TasteGroundingChips.tsx must exist')

  const reportSrc = fs.readFileSync(statusReportCardPath, 'utf8')
  const chipsSrc = fs.readFileSync(tasteChipsPath, 'utf8')

  // StatusReportCard uses calm hairline borders (border-l-2 instead of border-l-4)
  assert.ok(
    !reportSrc.includes('border-l-4'),
    'StatusReportCard must eliminate loud border-l-4 borders'
  )
  assert.ok(
    reportSrc.includes('border-l-2 border-l-emerald-500/70'),
    'StatusReportCard must use subtle border-l-2 border-l-emerald-500/70'
  )

  // TasteGroundingChips eliminates loud saturated cyan and amber borders
  assert.ok(
    !chipsSrc.includes('border-cyan-500/25') && !chipsSrc.includes('border-amber-500/25'),
    'TasteGroundingChips must eliminate saturated cyan and amber borders'
  )
  assert.ok(
    chipsSrc.includes('bg-muted/40 hover:bg-muted/70 text-muted-foreground hover:text-foreground border border-border/30'),
    'TasteGroundingChips must use calm muted chip aesthetics'
  )
})
