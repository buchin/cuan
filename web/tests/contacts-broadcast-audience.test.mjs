import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const contactsViewPath = path.join(webDir, 'src/features/contacts/ContactsView.tsx')
const broadcastPanelPath = path.join(webDir, 'src/features/contacts/BroadcastAudiencePanel.tsx')
const routesInboxPath = path.join(webDir, '../internal/server/routes_inbox.go')
const crmBroadcastPath = path.join(webDir, '../internal/server/crm_broadcast.go')

test('1. ContactsView: Broadcast button and panel integration', () => {
  const contactsViewSrc = fs.readFileSync(contactsViewPath, 'utf8')

  // Imports BroadcastAudiencePanel and Radio icon
  assert.ok(
    contactsViewSrc.includes('BroadcastAudiencePanel'),
    'ContactsView must import BroadcastAudiencePanel'
  )
  assert.ok(
    contactsViewSrc.includes('Radio'),
    'ContactsView must import Radio icon from lucide-react'
  )

  // Has Broadcast button in toolbar with aria-pressed
  assert.ok(
    contactsViewSrc.includes("label={t('Broadcast')}"),
    'ContactsView toolbar must have Broadcast IconButton'
  )
  assert.ok(
    contactsViewSrc.includes('onClick={toggleBroadcast}'),
    'ContactsView must bind toggleBroadcast on click'
  )

  // Search params sync
  assert.ok(
    contactsViewSrc.includes("params.get('view') === 'broadcast'"),
    'ContactsView must support ?view=broadcast in URL query params'
  )

  // Empty state has New broadcast shortcut button
  assert.ok(
    contactsViewSrc.includes("t('New broadcast')"),
    'ContactsView desktop empty canvas must offer New broadcast action'
  )

  // Renders BroadcastAudiencePanel when isBroadcast is active
  assert.ok(
    contactsViewSrc.includes('<BroadcastAudiencePanel'),
    'ContactsView must render BroadcastAudiencePanel when in broadcast mode'
  )
})

test('2. BroadcastAudiencePanel: Calm Software, Tag selector, Custom Email, and Audience Preview', () => {
  const broadcastPanelSrc = fs.readFileSync(broadcastPanelPath, 'utf8')

  // Calm Software Bear Notes typographic measure
  assert.ok(
    broadcastPanelSrc.includes('max-w-[75ch]'),
    'BroadcastAudiencePanel must adhere to 65-80ch Bear Note typographic measure'
  )
  assert.ok(
    broadcastPanelSrc.includes('border-border/40'),
    'BroadcastAudiencePanel must use single hairline dividers (border-border/40)'
  )

  // Tag segment selection
  assert.ok(
    broadcastPanelSrc.includes('/api/v1/contacts/broadcast/tags'),
    'BroadcastAudiencePanel must fetch available CRM tags summary'
  )
  assert.ok(
    broadcastPanelSrc.includes('Pilih Segmen Tag CRM'),
    'BroadcastAudiencePanel must feature CRM tag segment selector'
  )

  // Custom email input
  assert.ok(
    broadcastPanelSrc.includes('Alamat Email Kustom Tambahan'),
    'BroadcastAudiencePanel must have custom email input section'
  )
  assert.ok(
    broadcastPanelSrc.includes('custom-emails-textarea'),
    'BroadcastAudiencePanel must have custom emails textarea'
  )

  // Audience resolver endpoint call
  assert.ok(
    broadcastPanelSrc.includes('/api/v1/contacts/broadcast/audience'),
    'BroadcastAudiencePanel must query /api/v1/contacts/broadcast/audience'
  )

  // Pratinjau penerima & deduplikasi
  assert.ok(
    broadcastPanelSrc.includes('Pratinjau Penerima'),
    'BroadcastAudiencePanel must display Pratinjau Penerima section'
  )
  assert.ok(
    broadcastPanelSrc.includes('penerima unik'),
    'BroadcastAudiencePanel must display total penerima unik'
  )
  assert.ok(
    broadcastPanelSrc.includes('Deduplikasi otomatis'),
    'BroadcastAudiencePanel must display automatic deduplication indicator'
  )
})

test('3. Backend: Route registration and strict audience resolver invariants', () => {
  const routesInboxSrc = fs.readFileSync(routesInboxPath, 'utf8')
  const crmBroadcastSrc = fs.readFileSync(crmBroadcastPath, 'utf8')

  // Endpoints mounted under protected router pr
  assert.ok(
    routesInboxSrc.includes('pr.Post("/contacts/broadcast/audience", s.handleBroadcastAudience)'),
    'Audience resolver must be mounted on POST /contacts/broadcast/audience'
  )
  assert.ok(
    routesInboxSrc.includes('pr.Post("/contacts/broadcast/preview", s.handleBroadcastAudience)'),
    'Audience preview alias must be mounted on POST /contacts/broadcast/preview'
  )
  assert.ok(
    routesInboxSrc.includes('pr.Get("/contacts/broadcast/tags", s.handleBroadcastTags)'),
    'Tag summary endpoint must be mounted on GET /contacts/broadcast/tags'
  )

  // Strict validation & deduplication invariants
  assert.ok(
    crmBroadcastSrc.includes('parseAndValidateEmail'),
    'Backend must strictly validate recipient email addresses'
  )
  assert.ok(
    crmBroadcastSrc.includes('TotalUnique'),
    'Backend response must include TotalUnique count'
  )
  assert.ok(
    crmBroadcastSrc.includes('CRMCount') && crmBroadcastSrc.includes('CustomCount'),
    'Backend response must include breakdown of CRMCount and CustomCount'
  )
})
