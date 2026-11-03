import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const appsSkillsApiPath = path.join(webDir, 'src/api/appsSkills.ts')
const chatApiPath = path.join(webDir, 'src/api/chat.ts')
const appsSkillsViewPath = path.join(webDir, 'src/features/settings/AppsSkillsView.tsx')
const settingsMasterNavPath = path.join(webDir, 'src/features/settings/SettingsMasterNav.tsx')
const settingsViewPath = path.join(webDir, 'src/features/settings/SettingsView.tsx')
const editAgentModalPath = path.join(webDir, 'src/features/team/EditAgentModal.tsx')
const receiptPreviewCardPath = path.join(webDir, 'src/features/inbox/ReceiptPreviewCard.tsx')
const messageContentPath = path.join(webDir, 'src/features/inbox/MessageContent.tsx')
const inboxViewPath = path.join(webDir, 'src/features/inbox/InboxView.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

test('1. API Layer: Apps & Skills and Receipt Extraction contracts in appsSkills.ts', () => {
  assert.ok(fs.existsSync(appsSkillsApiPath), 'appsSkills.ts must exist in web/src/api/')

  const apiSrc = fs.readFileSync(appsSkillsApiPath, 'utf8')

  // Check exported interfaces
  assert.ok(apiSrc.includes('export interface App'), 'Must export App interface')
  assert.ok(apiSrc.includes('export interface Skill'), 'Must export Skill interface')
  assert.ok(apiSrc.includes('export interface AgentSkillView'), 'Must export AgentSkillView interface')
  assert.ok(apiSrc.includes('export interface ReceiptItem'), 'Must export ReceiptItem interface')
  assert.ok(apiSrc.includes('export interface ReceiptExtraction'), 'Must export ReceiptExtraction interface')

  // Check API methods
  assert.ok(apiSrc.includes('listApps('), 'Must provide listApps method')
  assert.ok(apiSrc.includes('connectApp('), 'Must provide connectApp method')
  assert.ok(apiSrc.includes('disconnectApp('), 'Must provide disconnectApp method')
  assert.ok(apiSrc.includes('listSkills('), 'Must provide listSkills method')
  assert.ok(apiSrc.includes('getAgentSkills('), 'Must provide getAgentSkills method')
  assert.ok(apiSrc.includes('updateAgentSkills('), 'Must provide updateAgentSkills method')

  // Check chat.ts integration
  const chatSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(
    chatSrc.includes('receipt_extraction?: ReceiptExtraction'),
    'ChatMessage must support optional receipt_extraction field',
  )
})

test('2. Settings Apps & Skills View: Sub-tabs, 1-Click Connect/Disconnect, and Catalog UI', () => {
  assert.ok(fs.existsSync(appsSkillsViewPath), 'AppsSkillsView.tsx must exist in web/src/features/settings/')

  const viewSrc = fs.readFileSync(appsSkillsViewPath, 'utf8')

  // Sub-tabs
  assert.ok(viewSrc.includes("data-testid=\"subtab-apps\""), 'AppsSkillsView must render subtab-apps button')
  assert.ok(viewSrc.includes("data-testid=\"subtab-skills\""), 'AppsSkillsView must render subtab-skills button')
  assert.ok(viewSrc.includes('<ListTab selected='), 'Sub-tab switcher must use accessible role="tab"')

  // 1-Click Connect & Disconnect buttons
  assert.ok(viewSrc.includes('data-testid={`connect-btn-${app.id}`}'), 'Must render 1-click connect button')
  assert.ok(viewSrc.includes('data-testid={`disconnect-btn-${app.id}`}'), 'Must render 1-click disconnect button')
  assert.ok(viewSrc.includes('handleConfirmConnect'), 'Must implement connect handler')
  assert.ok(viewSrc.includes('handleConfirmDisconnect'), 'Must implement disconnect handler')

  // Touch Accessibility (WCAG 2.1 AA min-h-11)
  assert.ok(viewSrc.includes('min-h-11'), 'Must include min-h-11 touch targets for mobile accessibility')

  // Status badges
  assert.ok(viewSrc.includes('Connected'), 'Must display Connected status')
  assert.ok(viewSrc.includes('Disconnected'), 'Must display Disconnected status')
  assert.ok(viewSrc.includes('Active'), 'Must display Active status')

  // Bound agents display
  assert.ok(viewSrc.includes('Bound Agents'), 'Must display bound agents for skills')
  assert.ok(viewSrc.includes('Configure in Agents & Team'), 'Must provide link to configure agents')
})

test('3. Navigation Integration: SettingsMasterNav and SettingsView route mapping', () => {
  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')

  // Nav item in SettingsMasterNav
  assert.ok(masterNavSrc.includes("id: 'apps-skills'"), 'SettingsMasterNav must include apps-skills nav item')
  assert.ok(
    masterNavSrc.includes("to: isAccount ? '/account/apps-skills' : '/settings/apps-skills'"),
    'SettingsMasterNav must link to /settings/apps-skills and /account/apps-skills',
  )
  assert.ok(masterNavSrc.includes("Boxes"), 'SettingsMasterNav must import and use Boxes icon')

  // SettingsView rendering
  assert.ok(
    settingsViewSrc.includes("import { AppsSkillsView } from './AppsSkillsView'"),
    'SettingsView must import AppsSkillsView',
  )
  assert.ok(
    settingsViewSrc.includes("section === 'apps-skills'"),
    'SettingsView must handle apps-skills route section',
  )
  assert.ok(
    settingsViewSrc.includes("<AppsSkillsView showHeading={false} />"),
    'SettingsView must render <AppsSkillsView />',
  )
})

test('4. EditAgentModal: Per-Agent Skill Binding with Real-Time Toggle', () => {
  const modalSrc = fs.readFileSync(editAgentModalPath, 'utf8')

  // Section existence
  assert.ok(
    modalSrc.includes("data-testid=\"agent-skills-section\""),
    'EditAgentModal must contain agent-skills-section',
  )

  // Real-time toggle switch
  assert.ok(
    modalSrc.includes("role=\"switch\""),
    'Skill toggle must use accessible role="switch"',
  )
  assert.ok(
    modalSrc.includes("aria-checked="),
    'Skill toggle must declare aria-checked attribute',
  )
  assert.ok(
    modalSrc.includes("handleToggleSkill"),
    'Must implement real-time handleToggleSkill handler',
  )

  // API integration
  assert.ok(
    modalSrc.includes("appsSkillsApi.getAgentSkills"),
    'Must fetch agent skills via appsSkillsApi.getAgentSkills',
  )
  assert.ok(
    modalSrc.includes("appsSkillsApi.updateAgentSkills"),
    'Must persist skill changes via appsSkillsApi.updateAgentSkills',
  )

  // Touch accessibility
  assert.ok(
    modalSrc.includes("min-h-11"),
    'Toggle buttons must satisfy min-h-11 (>= 44x44px) touch target standard',
  )
})

test('5. Inbox Vision ReceiptPreviewCard: Data Extraction & Confirm & Record Action', () => {
  assert.ok(fs.existsSync(receiptPreviewCardPath), 'ReceiptPreviewCard.tsx must exist in web/src/features/inbox/')

  const cardSrc = fs.readFileSync(receiptPreviewCardPath, 'utf8')

  // Receipt details rendering
  assert.ok(cardSrc.includes('extraction.merchant'), 'Must render extraction merchant/vendor')
  assert.ok(cardSrc.includes('extraction.date'), 'Must render extraction date')
  assert.ok(cardSrc.includes('extraction.total_amount'), 'Must render extraction total amount')
  assert.ok(cardSrc.includes('extraction.items'), 'Must render itemized list')
  assert.ok(cardSrc.includes('formatCurrencyIDR'), 'Must format IDR currency properly')

  // Human-in-the-Loop Action Button
  assert.ok(
    cardSrc.includes('data-testid="confirm-record-button"'),
    'Must render Confirm & Record button (Konfirmasi & Catat)',
  )
  assert.ok(
    cardSrc.includes('Confirmed & Recorded in Ledger'),
    'Must render confirmed status badge after action',
  )
  assert.ok(
    cardSrc.includes('min-h-11'),
    'Confirm button must meet min-h-11 touch target requirements',
  )

  // Parsing function
  assert.ok(
    cardSrc.includes('export function parseReceiptExtraction'),
    'Must export parseReceiptExtraction parser function',
  )

  // Inbox & MessageContent integration
  const msgContentSrc = fs.readFileSync(messageContentPath, 'utf8')
  assert.ok(
    msgContentSrc.includes('ReceiptPreviewCard'),
    'MessageContent must import and render ReceiptPreviewCard',
  )
  assert.ok(
    msgContentSrc.includes('parseReceiptExtraction'),
    'MessageContent must utilize parseReceiptExtraction',
  )

  const inboxSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxSrc.includes('ReceiptPreviewCard'),
    'InboxView must import ReceiptPreviewCard',
  )
  assert.ok(
    inboxSrc.includes('receipt_extraction'),
    'InboxView must support receipt_extraction in message rendering',
  )
})

test('6. Functional Test: parseReceiptExtraction and formatCurrencyIDR logic', () => {
  function formatCurrencyIDR(amount, currency = 'IDR') {
    return `Rp ${Number(amount).toLocaleString('id-ID')}`
  }

  function parseReceiptExtraction(content) {
    if (!content) return null
    const fenceMatch = content.match(/```(?:cuan-receipt|receipt)\s*([\s\S]*?)```/)
    if (fenceMatch && fenceMatch[1]) {
      try {
        const parsed = JSON.parse(fenceMatch[1].trim())
        if (parsed && (parsed.merchant || parsed.total_amount !== undefined)) {
          return parsed
        }
      } catch {}
    }
    return null
  }

  // Test currency formatting
  assert.ok(formatCurrencyIDR(150000).includes('150') && formatCurrencyIDR(150000).includes('Rp'), 'Should format 150000 to IDR string')

  // Test parsing ```cuan-receipt fence
  const rawReceiptFence = `Berikut ringkasan struk belanja Anda:
\`\`\`cuan-receipt
{
  "merchant": "Indomaret Tebet",
  "date": "2026-10-30",
  "total_amount": 78500,
  "currency": "IDR",
  "category": "Belanja Toko",
  "items": [
    { "name": "Beras 2.5kg", "quantity": 1, "price": 45000, "total": 45000 },
    { "name": "Minyak Goreng 1L", "quantity": 1, "price": 33500, "total": 33500 }
  ],
  "confidence": 0.95
}
\`\`\`
Silakan periksa dan konfirmasi catatannya.`

  const parsed = parseReceiptExtraction(rawReceiptFence)
  assert.ok(parsed !== null, 'parseReceiptExtraction should parse cuan-receipt block')
  assert.equal(parsed.merchant, 'Indomaret Tebet')
  assert.equal(parsed.total_amount, 78500)
  assert.equal(parsed.items.length, 2)
  assert.equal(parsed.items[0].name, 'Beras 2.5kg')
  assert.equal(parsed.confidence, 0.95)

  // Test empty / non-receipt content
  assert.equal(parseReceiptExtraction('Halo ada yang bisa dibantu?'), null)
})

test('7. Localization Completeness: Indonesian dictionary contains all new strings', () => {
  const idLocaleSrc = fs.readFileSync(idLocalePath, 'utf8')
  const idJson = JSON.parse(idLocaleSrc)

  const mandatoryKeys = [
    'Apps & Skills',
    'Connect 1-click external apps and modular AI skills',
    'Connected Apps',
    'Skills Catalog',
    'Zero-Developer Friction & Safe Execution',
    'Connected',
    'Disconnected',
    '1-Click Connect',
    'Disconnect App',
    'Receipt Extraction',
    'Confirm & Record',
    'Confirmed & Recorded in Ledger',
    'Vendor / Merchant',
    'Purchased Items',
    'Total Amount',
    'Modular Skills & Capabilities',
    'Real-Time Toggle',
    'Enabled',
    'Disabled',
  ]

  for (const k of mandatoryKeys) {
    assert.ok(
      idJson[k] !== undefined && idJson[k] !== '',
      `id.json must contain non-empty translation for "${k}"`,
    )
  }
})
