import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')
const repoRoot = path.resolve(webDir, '..')

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

test('4. Phase 2 UI: Message composer, 1-tap confirmation, and delivery status indicators', () => {
  const broadcastPanelSrc = fs.readFileSync(broadcastPanelPath, 'utf8')

  // Subject and Content inputs
  assert.ok(
    broadcastPanelSrc.includes('broadcast-subject-input'),
    'BroadcastAudiencePanel must have broadcast subject input field'
  )
  assert.ok(
    broadcastPanelSrc.includes('broadcast-content-textarea'),
    'BroadcastAudiencePanel must have broadcast message content textarea'
  )

  // Standard RFC 8058 & Unsubscribe notification in UI
  assert.ok(
    broadcastPanelSrc.includes('List-Unsubscribe (RFC 8058)'),
    'BroadcastAudiencePanel must explain automatic RFC 8058 unsubscribe header embedding'
  )

  // 1-tap send button with idempotency protection
  assert.ok(
    broadcastPanelSrc.includes('send-broadcast-button'),
    'BroadcastAudiencePanel must feature send-broadcast-button'
  )
  assert.ok(
    broadcastPanelSrc.includes('/api/v1/contacts/broadcast/send'),
    'BroadcastAudiencePanel must dispatch POST to /api/v1/contacts/broadcast/send'
  )
  assert.ok(
    broadcastPanelSrc.includes('idempotencyKeyRef'),
    'BroadcastAudiencePanel must maintain client-side idempotency key'
  )

  // Status indicators: sending, success, errors
  assert.ok(
    broadcastPanelSrc.includes('Sedang mengirim broadcast…'),
    'BroadcastAudiencePanel must display sending in-progress indicator'
  )
  assert.ok(
    broadcastPanelSrc.includes('Sukses terkirim ke'),
    'BroadcastAudiencePanel must display transparent success indicator with sent count'
  )
})

test('5. Phase 2 Backend: Batch sending, idempotency protection, and RFC 8058 unsubscribe endpoints', () => {
  const routesInboxSrc = fs.readFileSync(routesInboxPath, 'utf8')
  const crmBroadcastSrc = fs.readFileSync(crmBroadcastPath, 'utf8')

  // Batch send route registered under protected router
  assert.ok(
    routesInboxSrc.includes('pr.Post("/contacts/broadcast/send", s.handleBroadcastSend)'),
    'Batch send endpoint must be mounted on POST /contacts/broadcast/send'
  )

  // Public RFC 8058 unsubscribe endpoints
  assert.ok(
    routesInboxSrc.includes('r.Get("/api/v1/contacts/broadcast/unsubscribe", s.handleBroadcastUnsubscribe)'),
    'Unsubscribe GET endpoint must be mounted on /api/v1/contacts/broadcast/unsubscribe'
  )
  assert.ok(
    routesInboxSrc.includes('r.Post("/api/v1/contacts/broadcast/unsubscribe", s.handleBroadcastUnsubscribe)'),
    'Unsubscribe POST endpoint (RFC 8058) must be mounted on /api/v1/contacts/broadcast/unsubscribe'
  )

  // In-flight locking and idempotency cache
  assert.ok(
    crmBroadcastSrc.includes('sentBroadcastCache'),
    'Backend must maintain sent broadcast cache for duplicate protection'
  )
  assert.ok(
    crmBroadcastSrc.includes('inFlightBroadcasts'),
    'Backend must maintain in-flight concurrency lock'
  )

  // RFC 8058 headers
  assert.ok(
    crmBroadcastSrc.includes('"List-Unsubscribe"'),
    'Backend must embed List-Unsubscribe header'
  )
  assert.ok(
    crmBroadcastSrc.includes('"List-Unsubscribe-Post": "List-Unsubscribe=One-Click"'),
    'Backend must embed RFC 8058 List-Unsubscribe-Post header'
  )
})

test('6. Phase 3 UI: Broadcast history tab, session list, and SlideDrawer recipient logs', () => {
  const broadcastPanelSrc = fs.readFileSync(broadcastPanelPath, 'utf8')

  // Tab navigation between Compose and History
  assert.ok(
    broadcastPanelSrc.includes('id="broadcast-tab-compose"'),
    'BroadcastAudiencePanel must have Tulis Broadcast tab'
  )
  assert.ok(
    broadcastPanelSrc.includes('id="broadcast-tab-history"'),
    'BroadcastAudiencePanel must have Riwayat Pengiriman tab'
  )

  // Queries broadcast history endpoint
  assert.ok(
    broadcastPanelSrc.includes('/api/v1/contacts/broadcast/history'),
    'BroadcastAudiencePanel must fetch history from /api/v1/contacts/broadcast/history'
  )

  // History list panel with calm software aesthetics
  assert.ok(
    broadcastPanelSrc.includes('id="broadcast-panel-history"'),
    'BroadcastAudiencePanel must have history panel container'
  )
  assert.ok(
    broadcastPanelSrc.includes('Daftar Riwayat Pengiriman'),
    'BroadcastAudiencePanel must display Daftar Riwayat Pengiriman'
  )

  // SlideDrawer integration for detailed session inspection (DoD 4)
  assert.ok(
    broadcastPanelSrc.includes('SlideDrawer'),
    'BroadcastAudiencePanel must integrate SlideDrawer for session details'
  )
  assert.ok(
    broadcastPanelSrc.includes('data-testid="broadcast-history-detail-drawer"'),
    'BroadcastAudiencePanel must mount broadcast-history-detail-drawer SlideDrawer'
  )
  assert.ok(
    broadcastPanelSrc.includes('Status Tiap Penerima'),
    'BroadcastAudiencePanel must display recipient delivery status section'
  )
  assert.ok(
    broadcastPanelSrc.includes('recipientFilter'),
    'BroadcastAudiencePanel must allow filtering recipient logs by status'
  )
  assert.ok(
    broadcastPanelSrc.includes('error_message'),
    'BroadcastAudiencePanel must display error message on delivery failures'
  )
})

test('7. Phase 3 Backend: SQL schema migration 000043, session storage, and history endpoints', () => {
  const routesInboxSrc = fs.readFileSync(routesInboxPath, 'utf8')
  const crmBroadcastSrc = fs.readFileSync(crmBroadcastPath, 'utf8')

  // History endpoints mounted in routes_inbox
  assert.ok(
    routesInboxSrc.includes('pr.Get("/contacts/broadcast/history", s.handleBroadcastHistory)'),
    'History list endpoint must be mounted on GET /contacts/broadcast/history'
  )
  assert.ok(
    routesInboxSrc.includes('pr.Get("/contacts/broadcast/history/{id}", s.handleBroadcastHistoryDetail)'),
    'History detail endpoint must be mounted on GET /contacts/broadcast/history/{id}'
  )

  // Data structures and storage methods
  assert.ok(
    crmBroadcastSrc.includes('type CRMBroadcastSession struct'),
    'Backend must define CRMBroadcastSession struct'
  )
  assert.ok(
    crmBroadcastSrc.includes('type CRMBroadcastLog struct'),
    'Backend must define CRMBroadcastLog struct'
  )
  assert.ok(
    crmBroadcastSrc.includes('saveBroadcastSession'),
    'Backend must implement saveBroadcastSession'
  )
  assert.ok(
    crmBroadcastSrc.includes('listBroadcastSessions'),
    'Backend must implement listBroadcastSessions'
  )
  assert.ok(
    crmBroadcastSrc.includes('getBroadcastSessionDetail'),
    'Backend must implement getBroadcastSessionDetail'
  )

  // Migration 000043 files exist and create tables
  const upSqlPath = path.join(repoRoot, 'migrations/000043_crm_broadcast_history.up.sql')
  const downSqlPath = path.join(repoRoot, 'migrations/000043_crm_broadcast_history.down.sql')
  assert.ok(fs.existsSync(upSqlPath), '000043_crm_broadcast_history.up.sql must exist')
  assert.ok(fs.existsSync(downSqlPath), '000043_crm_broadcast_history.down.sql must exist')

  const upSql = fs.readFileSync(upSqlPath, 'utf8')
  assert.ok(upSql.includes('crm_broadcast_sessions'), 'up.sql must create crm_broadcast_sessions')
  assert.ok(upSql.includes('crm_broadcast_logs'), 'up.sql must create crm_broadcast_logs')
})
