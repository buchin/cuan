import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const messageContentPath = path.resolve(__dirname, '../src/features/inbox/MessageContent.tsx')
const statusReportCardPath = path.resolve(__dirname, '../src/components/ui/StatusReportCard.tsx')

test('1. MessageContent imports and renders StatusReportCard for task and DoD reports', () => {
  const src = fs.readFileSync(messageContentPath, 'utf8')
  assert.ok(
    src.includes('StatusReportCard') && src.includes('parseTaskReportMessage'),
    'MessageContent must import StatusReportCard and parseTaskReportMessage'
  )
  assert.ok(
    src.includes('parseTaskReportMessage(body, language)'),
    'MessageContent must parse body for task/DoD reports'
  )
  assert.ok(
    src.includes('<StatusReportCard report={report}'),
    'MessageContent must render StatusReportCard component when report is parsed'
  )
})

test('2. StatusReportCard exports rich DoD report interfaces and theme configurations', () => {
  const src = fs.readFileSync(statusReportCardPath, 'utf8')
  assert.ok(src.includes('export interface DoDItem'), 'Must export DoDItem interface')
  assert.ok(src.includes('export interface DoDTechnicalVerification'), 'Must export DoDTechnicalVerification')
  assert.ok(src.includes('export interface DoDReportDetails'), 'Must export DoDReportDetails')
  assert.ok(src.includes('export interface ParsedTaskReport'), 'Must export ParsedTaskReport')
  assert.ok(src.includes('parseTaskReportMessage'), 'Must export parseTaskReportMessage')
  assert.ok(src.includes('StatusReportCard'), 'Must export StatusReportCard')
})

test('3. StatusReportCard uses shadcn UI patterns and Lucide icons for rich DoD cards', () => {
  const src = fs.readFileSync(statusReportCardPath, 'utf8')
  const expectedIcons = [
    'ShieldCheck',
    'CheckCircle2',
    'Clock3',
    'AlertCircle',
    'Terminal',
    'GitCommit',
    'GitBranch',
    'Rocket',
    'ListChecks',
    'AlertTriangle',
    'CheckSquare',
  ]
  for (const icon of expectedIcons) {
    assert.ok(src.includes(icon), `StatusReportCard must use Lucide icon: ${icon}`)
  }
  assert.ok(src.includes('data-report-status='), 'Card must have data-report-status attribute for CSS targeting')
  assert.ok(src.includes('rounded-xl border'), 'Card must use shadcn rounded-xl border aesthetics')
})

test('4. StatusReportCard truncates 40-character commit hashes to 7 chars and handles explicit no-gap', () => {
  const src = fs.readFileSync(statusReportCardPath, 'utf8')
  assert.ok(
    src.includes('slice(0, 7)'),
    'StatusReportCard must truncate commit hashes to short 7 characters'
  )
  assert.ok(
    src.includes('hasExplicitNoGap'),
    'StatusReportCard must handle explicit no-gap state without rendering false empty checkbox'
  )
})

test('5. InboxView suppresses cuan_work_review and obsolete stopped launch notifications', () => {
  const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
  const src = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    src.includes("msg.author_id === 'cuan_work_review'"),
    'InboxView must suppress cuan_work_review internal prompt'
  )
  assert.ok(
    src.includes("msgContent.includes('This pending launch has stopped')"),
    'InboxView must check and suppress obsolete stopped launch notices'
  )
})
