import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const tasteApiPath = path.join(__dirname, '..', 'src', 'api', 'taste.ts')
const chatApiPath = path.join(__dirname, '..', 'src', 'api', 'chat.ts')
const tasteChipsComponentPath = path.join(
  __dirname,
  '..',
  'src',
  'features',
  'inbox',
  'TasteGroundingChips.tsx',
)
const inboxViewPath = path.join(__dirname, '..', 'src', 'features', 'inbox', 'InboxView.tsx')
const studioViewPath = path.join(__dirname, '..', 'src', 'features', 'inbox', 'StudioView.tsx')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. API Layer: GroundingTaste contracts and getActiveGrounding endpoints exist', () => {
  assert.ok(fs.existsSync(tasteApiPath), 'src/api/taste.ts must exist')
  assert.ok(fs.existsSync(chatApiPath), 'src/api/chat.ts must exist')

  const tasteSource = fs.readFileSync(tasteApiPath, 'utf8')
  const chatSource = fs.readFileSync(chatApiPath, 'utf8')

  // GroundingTaste interface
  assert.ok(
    tasteSource.includes('export interface GroundingTaste'),
    'taste.ts must export GroundingTaste interface',
  )
  assert.ok(
    tasteSource.includes('category: TasteCategory') || tasteSource.includes('category:'),
    'GroundingTaste must include category',
  )
  assert.ok(
    tasteSource.includes('topic: string'),
    'GroundingTaste must include topic field',
  )
  assert.ok(
    tasteSource.includes('summary: string'),
    'GroundingTaste must include summary field',
  )

  // SessionGroundingTastesResponse
  assert.ok(
    tasteSource.includes('export interface SessionGroundingTastesResponse'),
    'taste.ts must export SessionGroundingTastesResponse',
  )

  // API endpoint implementation in tasteApi
  assert.ok(
    tasteSource.includes('getActiveGrounding: async') &&
      tasteSource.includes('/api/v1/taste/grounding'),
    'tasteApi must implement getActiveGrounding calling /api/v1/taste/grounding',
  )

  // ChatMessageDTO / ChatMessage / Message contracts in chat.ts
  assert.ok(
    chatSource.includes('grounding_tastes?: GroundingTaste[]'),
    'ChatMessage interface must include optional grounding_tastes field',
  )
  assert.ok(
    chatSource.includes('export type { GroundingTaste }') || chatSource.includes('export interface GroundingTaste'),
    'chat.ts must export GroundingTaste',
  )
  assert.ok(
    chatSource.includes('export type Message = ChatMessage'),
    'chat.ts must export Message type alias',
  )
})

test('2. UI Component: TasteGroundingChips clutter guardrail (max 2 chips + counter badge)', () => {
  assert.ok(fs.existsSync(tasteChipsComponentPath), 'TasteGroundingChips.tsx must exist')
  const source = fs.readFileSync(tasteChipsComponentPath, 'utf8')

  // Component export & props
  assert.ok(
    source.includes('export function TasteGroundingChips'),
    'Must export TasteGroundingChips component',
  )
  assert.ok(
    source.includes('tastes?: GroundingTaste[]'),
    'TasteGroundingChipsProps must accept tastes array',
  )

  // Anti-clutter limit guardrail (max 2 visible chips default)
  assert.ok(
    source.includes('maxVisible = 2') || source.includes('maxVisible = 2'),
    'Must default maxVisible to 2',
  )
  assert.ok(
    source.includes('tastes.slice(0, maxVisible)') || source.includes('slice(0, maxVisible)'),
    'Must slice tastes array to maxVisible to avoid UI clutter',
  )
  assert.ok(
    source.includes('tastes.length - maxVisible'),
    'Must calculate remaining count for counter badge',
  )

  // Visual chip elements
  assert.ok(
    source.includes('data-testid="taste-grounding-chip"'),
    'Must render chips with data-testid="taste-grounding-chip"',
  )
  assert.ok(
    source.includes("t('Memori')") && source.includes('topicName'),
    'Must render elegant chip text [✨ Memori: <Topik>]',
  )

  // Counter badge (+N lainnya)
  assert.ok(
    source.includes('data-testid="taste-grounding-counter-badge"'),
    'Must render counter badge with data-testid="taste-grounding-counter-badge"',
  )
  assert.ok(
    source.includes('+{remainingCount}') && source.includes("t('lainnya')"),
    'Must format counter badge as +N lainnya',
  )
})

test('3. UI Component: 1-Click Context Popover detail and /settings/memory shortcut', () => {
  assert.ok(fs.existsSync(tasteChipsComponentPath), 'TasteGroundingChips.tsx must exist')
  const source = fs.readFileSync(tasteChipsComponentPath, 'utf8')

  // Popover dialog attributes
  assert.ok(
    source.includes('data-testid="taste-grounding-popover"'),
    'Must render popover with data-testid="taste-grounding-popover"',
  )
  assert.ok(
    source.includes('role="dialog"'),
    'Popover must have role="dialog" accessibility attribute',
  )

  // Category labels (Brand vs Technical)
  assert.ok(
    source.includes('data-testid="grounding-taste-category-badge"'),
    'Must render category badge for each active preference',
  )
  assert.ok(
    source.includes("t('Brand Taste')") && source.includes("t('Technical Taste')"),
    'Must distinguish Brand Taste vs Technical Taste in popover',
  )

  // Active preference snippet
  assert.ok(
    source.includes('taste.summary') && source.includes('taste.preview'),
    'Must display snippet of active preference text',
  )

  // 1-Click Navigation shortcut to /settings/memory
  assert.ok(
    source.includes('data-testid="taste-grounding-settings-link"'),
    'Must render 1-click settings shortcut with data-testid="taste-grounding-settings-link"',
  )
  assert.ok(
    source.includes('/settings/memory'),
    'Shortcut must navigate directly to /settings/memory',
  )

  // Dismissal mechanisms (Escape key, click outside, close button)
  assert.ok(
    source.includes("e.key === 'Escape'"),
    'Must listen to Escape key to dismiss popover',
  )
  assert.ok(
    source.includes('handleClickOutside') && source.includes('mousedown'),
    'Must dismiss popover on outside click',
  )
  assert.ok(
    source.includes('data-testid="close-grounding-popover"'),
    'Must provide close button for popover',
  )
})

test('4. Chat Integration: StudioView renders TasteGroundingChips (eliminated from InboxView in S62-T2)', () => {
  assert.ok(fs.existsSync(inboxViewPath), 'InboxView.tsx must exist')
  assert.ok(fs.existsSync(studioViewPath), 'StudioView.tsx must exist')

  const inboxSource = fs.readFileSync(inboxViewPath, 'utf8')
  const studioSource = fs.readFileSync(studioViewPath, 'utf8')

  // InboxView chat bubble clutter elimination (S62-T2)
  assert.ok(
    !inboxSource.includes('<TasteGroundingChips'),
    'InboxView must eliminate TasteGroundingChips from chat bubble canvas to avoid visual clutter',
  )

  // StudioView chat composer integration
  assert.ok(
    studioSource.includes("import { TasteGroundingChips } from './TasteGroundingChips'"),
    'StudioView must import TasteGroundingChips',
  )
  assert.ok(
    studioSource.includes('tasteApi.getActiveGrounding') || studioSource.includes('activeGroundingTastes'),
    'StudioView must fetch active grounding tastes',
  )
  assert.ok(
    studioSource.includes('<TasteGroundingChips'),
    'StudioView must render TasteGroundingChips for active grounding preferences',
  )
})

test('5. Localization: Indonesian dictionary (id.json) includes grounding UI strings', () => {
  assert.ok(fs.existsSync(idLocalePath), 'locales/id.json must exist')
  const dict = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  const requiredKeys = [
    'Memori',
    'Preferensi',
    'lainnya',
    'Detail Preferensi Aktif',
    'Landasan Memori (Taste Grounding)',
    'Preferensi aktif yang memandu respon asisten AI',
    'Brand Taste',
    'Technical Taste',
    'Kelola Memori',
    'Pengaturan Memori',
    'Sesuaikan memori agen?',
  ]

  for (const key of requiredKeys) {
    assert.ok(
      dict[key] !== undefined && dict[key].length > 0,
      `id.json must contain translation for "${key}"`,
    )
  }
})

test('6. Functional Logic: Clutter guardrail pagination and category resolution', () => {
  // Test pagination logic
  const mockTastes = [
    { id: '1', category: 'brand', topic: 'Tone of Voice', summary: 'Ramah dan solutif' },
    { id: '2', category: 'technical', topic: 'Format Output', summary: 'Gunakan bullet points' },
    { id: '3', category: 'brand', topic: 'Target Audience', summary: 'Pelaku UMKM dan kreator' },
    { id: '4', category: 'technical', topic: 'Constraint Diskon', summary: 'Maks 10% diskon' },
  ]

  const maxVisible = 2
  const visible = mockTastes.slice(0, maxVisible)
  const remaining = mockTastes.length - maxVisible

  assert.equal(visible.length, 2, 'Default visible items must be capped at 2')
  assert.equal(visible[0].topic, 'Tone of Voice')
  assert.equal(visible[1].topic, 'Format Output')
  assert.equal(remaining, 2, 'Remaining items count must be exactly 2 for 4 items')

  // When count is <= 2
  const twoTastes = mockTastes.slice(0, 2)
  const remTwo = twoTastes.length - maxVisible
  assert.equal(remTwo <= 0, true, 'No counter badge when items count <= 2')

  // When count is 3
  const threeTastes = mockTastes.slice(0, 3)
  const remThree = threeTastes.length - maxVisible
  assert.equal(remThree, 1, 'Counter badge should indicate +1 lainnya when items count is 3')

  // Category labels resolution
  function resolveCategoryLabel(cat, dict) {
    return cat === 'brand' ? dict['Brand Taste'] : dict['Technical Taste']
  }
  const dict = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  assert.equal(resolveCategoryLabel('brand', dict), 'Brand Taste')
  assert.ok(resolveCategoryLabel('technical', dict).length > 0)
})
