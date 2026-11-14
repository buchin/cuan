import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveAttachmentUrl } from '../src/components/dialogs/filePreviewUtils.ts'

test('resolveAttachmentUrl correctly normalizes attachment URLs', () => {
  // 1. Legacy cuan.dojo.cc URL is rewritten to same-origin relative path
  assert.equal(
    resolveAttachmentUrl({
      id: '0f8ee5d0-b50a-407a-af6d-eddea20d1e55',
      url: 'https://cuan.dojo.cc/api/chat/attachments/0f8ee5d0-b50a-407a-af6d-eddea20d1e55',
    }),
    '/api/chat/attachments/0f8ee5d0-b50a-407a-af6d-eddea20d1e55',
  )

  // 2. Legacy URL with query params
  assert.equal(
    resolveAttachmentUrl({
      id: '212d3faf-afaf-4c18-a160-e3947e81180f',
      url: 'https://cuan.dojo.cc/api/chat/attachments/212d3faf-afaf-4c18-a160-e3947e81180f?preview=1',
    }),
    '/api/chat/attachments/212d3faf-afaf-4c18-a160-e3947e81180f',
  )

  // 3. Already relative URLs remain untouched
  assert.equal(
    resolveAttachmentUrl({
      id: 'att-123',
      url: '/api/v1/chat/attachments/att-123',
    }),
    '/api/v1/chat/attachments/att-123',
  )
  assert.equal(
    resolveAttachmentUrl({
      id: 'att-456',
      url: '/api/chat/attachments/att-456',
    }),
    '/api/chat/attachments/att-456',
  )

  // 4. Missing url but valid id falls back to /api/chat/attachments/{id}
  assert.equal(
    resolveAttachmentUrl({
      id: 'att-789',
    }),
    '/api/chat/attachments/att-789',
  )

  // 5. Cross-origin absolute URL without matching origin normalizes to pathname + search
  assert.equal(
    resolveAttachmentUrl({
      id: 'att-abc',
      url: 'https://other-domain.com/api/chat/attachments/att-abc?preview=1',
    }),
    '/api/chat/attachments/att-abc?preview=1',
  )

  // 6. Null or empty input
  assert.equal(resolveAttachmentUrl(), '')
  assert.equal(resolveAttachmentUrl(null), '')
})
