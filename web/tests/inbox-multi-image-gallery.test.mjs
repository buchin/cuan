import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const attachmentsPath = path.resolve(__dirname, '../src/features/inbox/Attachments.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatCalmCssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')

const attachmentsSrc = fs.readFileSync(attachmentsPath, 'utf8')
const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const chatCalmCssSrc = fs.readFileSync(chatCalmCssPath, 'utf8')

test('1. Multi-image detection in InboxView.tsx and Attachments.tsx', () => {
  // InboxView must check for multiple images before rendering
  assert.ok(
    inboxViewSrc.includes('allAttachments.filter(isImageAttachment).length > 1'),
    'InboxView.tsx must detect when a message has more than 1 image attachment'
  )
  assert.ok(
    inboxViewSrc.includes('<AttachmentGallery'),
    'InboxView.tsx must render AttachmentGallery for multi-image messages'
  )
  // Attachments.tsx must check for multiple images
  assert.ok(
    attachmentsSrc.includes('attachments.filter(isImageAttachment).length > 1'),
    'Attachments.tsx must detect when attachments contain more than 1 image'
  )
})

test('2. Adaptive Grid Gallery layouts for 2, 3, 4, and 5 images', () => {
  // Export verification
  assert.ok(
    attachmentsSrc.includes('export function AttachmentGallery'),
    'Attachments.tsx must export AttachmentGallery component'
  )
  assert.ok(
    attachmentsSrc.includes('export function AttachmentGalleryCell'),
    'Attachments.tsx must export AttachmentGalleryCell component'
  )

  // 2 images: 2 columns side by side
  assert.ok(
    attachmentsSrc.includes('grid-cols-2') && attachmentsSrc.includes('h-[160px]'),
    '2 images layout must use 2-column grid with proportional aspect ratio'
  )

  // 3 images: 1 main image + 2 side images (row-span-2)
  assert.ok(
    attachmentsSrc.includes('col-span-1 row-span-2 h-full'),
    '3 images layout must feature 1 main image spanning 2 rows alongside 2 stacked images'
  )

  // 4 images: 2x2 grid
  assert.ok(
    attachmentsSrc.includes('grid-cols-2 grid-rows-2'),
    '4 images layout must use a compact 2x2 grid'
  )

  // 5 images: 2 top + 3 bottom in 6-column grid
  assert.ok(
    attachmentsSrc.includes('grid-cols-6 grid-rows-2') &&
    attachmentsSrc.includes('col-span-3 row-span-1') &&
    attachmentsSrc.includes('col-span-2 row-span-1'),
    '5 images layout must use 2 top + 3 bottom cells in a 6-column grid'
  )
})

test('3. Tap/click interaction opens FilePreviewModal with correct attachment', () => {
  assert.ok(
    attachmentsSrc.includes('setPreviewAttachment(img)'),
    'AttachmentGallery must set previewAttachment when an image cell is clicked'
  )
  assert.ok(
    attachmentsSrc.includes('<FilePreviewModal'),
    'AttachmentGallery must render FilePreviewModal when previewAttachment is set'
  )
  assert.ok(
    attachmentsSrc.includes('onDownload={() => download(previewAttachment)}'),
    'AttachmentGallery must bind download action to FilePreviewModal'
  )
  assert.ok(
    attachmentsSrc.includes("aria-label={`${t('Preview')} ${a.name}`}"),
    'AttachmentGalleryCell must provide accessible Preview aria-label'
  )
})

test('4. Non-image files (PDF/docs) remain accessible and unaffected by gallery layout', () => {
  // Non-image documents are filtered and rendered in standard file rows below the gallery
  assert.ok(
    attachmentsSrc.includes('const docs = attachments.filter((a) => !isImageAttachment(a))'),
    'AttachmentGallery must filter non-image documents'
  )
  assert.ok(
    attachmentsSrc.includes('<AttachmentFile key={doc.id} attachment={doc} internal={internal} />'),
    'AttachmentGallery must render documents using standard AttachmentFile component'
  )
})

test('5. Responsive styling on mobile (390px) and desktop without overflow', () => {
  assert.ok(
    attachmentsSrc.includes('max-w-[320px]'),
    'AttachmentGallery must constrain max-width to 320px to fit mobile and desktop cleanly'
  )
  assert.ok(
    attachmentsSrc.includes('rounded-xl overflow-hidden'),
    'AttachmentGallery must have smooth rounded corners and prevent content overflow'
  )
  assert.ok(
    chatCalmCssSrc.includes('z-index: 10'),
    'chat-calm.css wa-markdown-footer must stay elevated above image thumbnails in imageOnly mode'
  )
})
