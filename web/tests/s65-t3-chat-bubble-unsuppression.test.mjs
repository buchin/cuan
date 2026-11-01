import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const messageContentPath = path.resolve(__dirname, '../src/features/inbox/MessageContent.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')

test('1. InboxView: eliminated suppression filter of intermediate runner progress and task links', () => {
  const src = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    !src.includes("!linkedTaskForMsg &&") || !src.includes("(msgContent.includes('[View work](/tasks/') || msgContent.includes('[Lihat tugas](/tasks/'))"),
    'InboxView must not drop agent messages that contain [View work] or [Lihat tugas] links'
  )
})

test('2. MessageContent: unsuppressed bubbles and fallback prevents empty bubbles', () => {
  const src = fs.readFileSync(messageContentPath, 'utf8')
  assert.ok(
    src.includes('effectiveActions'),
    'MessageContent must define effectiveActions to preserve action links when body is empty'
  )
  assert.ok(
    src.includes('effectiveBody'),
    'MessageContent must define effectiveBody to prevent blank bubbles'
  )
})

test('3. MessageContent: checks both body and content for parseTaskReportMessage and parseReceiptExtraction', () => {
  const src = fs.readFileSync(messageContentPath, 'utf8')
  assert.ok(
    src.includes('parseTaskReportMessage(content, language)'),
    'MessageContent must inspect content as fallback for task reports'
  )
  assert.ok(
    src.includes('parseReceiptExtraction(content)'),
    'MessageContent must inspect content as fallback for receipt extraction'
  )
})
