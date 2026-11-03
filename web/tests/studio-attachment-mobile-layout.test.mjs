import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const studioViewPath = path.resolve(__dirname, '../src/features/inbox/StudioView.tsx')
const attachmentsPath = path.resolve(__dirname, '../src/features/inbox/Attachments.tsx')
const selectorPath = path.resolve(__dirname, '../src/features/inbox/StudioExecutionSelector.tsx')

test('1. StudioView renders AttachmentPreviews in dedicated preview row above toolbar', () => {
  const src = fs.readFileSync(studioViewPath, 'utf8')
  assert.ok(
    src.includes('<AttachmentPreviews uploads={uploads} disabled={busy} />'),
    'StudioView must render AttachmentPreviews in dedicated preview container'
  )
  const previewsIdx = src.indexOf('<AttachmentPreviews')
  const toolbarIdx = src.indexOf('<StudioProjectSelector')
  assert.ok(
    previewsIdx !== -1 && previewsIdx < toolbarIdx,
    'AttachmentPreviews must be rendered ABOVE the bottom toolbar'
  )
})

test('2. StudioView uses compact AttachmentPicker beside execution selector', () => {
  const src = fs.readFileSync(studioViewPath, 'utf8')
  assert.ok(
    src.includes('<AttachmentPicker') && src.includes('compact={true}'),
    'StudioView must pass compact={true} to AttachmentPicker so previews do not inject into toolbar'
  )
})

test('3. StudioView toolbar prevents flex collision between selector and action button', () => {
  const src = fs.readFileSync(studioViewPath, 'utf8')
  assert.ok(
    src.includes('min-w-0 flex-1'),
    'Left toolbar container must have min-w-0 flex-1 to allow clean truncation'
  )
  assert.ok(
    src.includes('action-primary shrink-0'),
    'Submit button must be shrink-0 so it is never compressed or overlapped'
  )
})

test('4. Attachments truncates filenames gracefully without single-character breaking', () => {
  const src = fs.readFileSync(attachmentsPath, 'utf8')
  assert.ok(
    src.includes('truncate max-w-'),
    'AttachmentFile must truncate long filenames with ellipsis'
  )
  assert.ok(
    !src.includes('<span className="min-w-0 break-all">'),
    'AttachmentFile must not use break-all without truncation which collapses into vertical single-character columns'
  )
})

test('5. StudioExecutionSelector button has min-w-0 and max-w-full to prevent mobile overflow', () => {
  const src = fs.readFileSync(selectorPath, 'utf8')
  assert.ok(
    src.includes('max-w-full'),
    'Pill button must have max-w-full to prevent pushing against right-side actions'
  )
  assert.ok(
    src.includes('min-w-0'),
    'Pill container must have min-w-0 for flex parent truncation'
  )
})
