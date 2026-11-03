import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const tasteApiPath = path.join(__dirname, '..', 'src', 'api', 'taste.ts')
const agentMemoryViewPath = path.join(
  __dirname,
  '..',
  'src',
  'features',
  'settings',
  'AgentMemoryView.tsx',
)
const settingsViewPath = path.join(
  __dirname,
  '..',
  'src',
  'features',
  'settings',
  'SettingsView.tsx',
)
const studioViewPath = path.join(
  __dirname,
  '..',
  'src',
  'features',
  'inbox',
  'StudioView.tsx',
)
const appPath = path.join(__dirname, '..', 'src', 'App.tsx')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. API Layer: Taste API client and data contracts exist', () => {
  assert.ok(fs.existsSync(tasteApiPath), 'src/api/taste.ts must exist')
  const source = fs.readFileSync(tasteApiPath, 'utf8')

  assert.ok(
    source.includes('export type TasteCategory = \'brand\' | \'technical\''),
    'Must define TasteCategory type with brand and technical',
  )
  assert.ok(
    source.includes('export interface TasteItem'),
    'Must define TasteItem interface',
  )
  assert.ok(
    source.includes('export interface TasteProfile'),
    'Must define TasteProfile interface',
  )
  assert.ok(
    source.includes('export interface CreateTasteItemRequest'),
    'Must define CreateTasteItemRequest interface',
  )
  assert.ok(
    source.includes('export interface UpdateTasteItemRequest'),
    'Must define UpdateTasteItemRequest interface',
  )

  // Methods
  assert.ok(
    source.includes('getProfile: async (): Promise<TasteProfile>'),
    'tasteApi must implement getProfile (/api/v1/taste)',
  )
  assert.ok(
    source.includes('createItem: async (data: CreateTasteItemRequest)'),
    'tasteApi must implement createItem (/api/v1/taste/items)',
  )
  assert.ok(
    source.includes('updateItem: async (id: string, data: UpdateTasteItemRequest)'),
    'tasteApi must implement updateItem (/api/v1/taste/items/:id)',
  )
  assert.ok(
    source.includes('deleteItem: async (id: string)'),
    'tasteApi must implement deleteItem (/api/v1/taste/items/:id)',
  )
  assert.ok(
    source.includes('resetProfile: async (): Promise<TasteProfile>'),
    'tasteApi must implement resetProfile (/api/v1/taste/reset)',
  )
})

test('2. UI Component: AgentMemoryView component architecture and non-developer UX', () => {
  assert.ok(
    fs.existsSync(agentMemoryViewPath),
    'AgentMemoryView.tsx must exist',
  )
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  assert.ok(
    source.includes('export function AgentMemoryView'),
    'Must export AgentMemoryView component',
  )

  // Non-developer category representation
  assert.ok(
    source.includes('Brand & Voice') || source.includes('Brand Voice'),
    'Must display Brand & Voice category for non-developers',
  )
  assert.ok(
    source.includes('Technical & Workflows') || source.includes('Technical & Workflow'),
    'Must display Technical & Workflows category for non-developers',
  )

  // Context safety limit indicator
  assert.ok(
    source.includes('/ 15') && source.includes('Context Window Guard'),
    'Must display context limit guard (15 items) to prevent prompt bloat',
  )

  // Granular Actions
  assert.ok(
    source.includes('Lupakan poin ini'),
    'Must have granular forget action ("Lupakan poin ini")',
  )
  assert.ok(
    source.includes('handleOpenEdit'),
    'Must have granular edit handler',
  )
  assert.ok(
    source.includes('handleOpenAdd'),
    'Must have manual add preference handler',
  )

  // Safe Total Reset
  assert.ok(
    source.includes('Reset All Memory') || source.includes('Reset Semua Memori'),
    'Must provide Reset All Memory action',
  )
  assert.ok(
    source.includes('Restore default memory templates') ||
      source.includes('Reset Semua Memori Agen?'),
    'Must have safe confirmation dialog for total reset',
  )

  // Accessibility & touch targets
  assert.ok(
    source.includes('min-h-11'),
    'Must conform to WCAG 2.1 AA touch target standards (min-h-11 >= 44px)',
  )
  assert.ok(
    source.includes('ActionDialog'),
    'Must use accessible ActionDialog modals',
  )
})

test('3. Settings & Chat Integration: Navigation tabs, route titles and quick access', () => {
  const settingsSource = fs.readFileSync(settingsViewPath, 'utf8')
  const appSource = fs.readFileSync(appPath, 'utf8')
  const studioSource = fs.readFileSync(studioViewPath, 'utf8')

  // SettingsView tab navigation
  assert.ok(
    settingsSource.includes("['memory', t('Agent Memory')]"),
    'SettingsView must include memory in navigation tabs',
  )
  assert.ok(
    settingsSource.includes('section === \'memory\'') &&
      settingsSource.includes('<AgentMemoryView'),
    'SettingsView must render AgentMemoryView when section === "memory"',
  )
  assert.ok(
    fs.readFileSync(new URL('../src/features/settings/SettingsMasterNav.tsx', import.meta.url), 'utf8').includes("to: isAccount ? '/account/memory' : '/settings/memory'"),
    'SettingsView must link to /settings/memory in General settings card',
  )

  // App.tsx header titles
  assert.ok(
    appSource.includes("memory: 'Agent Memory'"),
    'App.tsx settingTitles must map memory to "Agent Memory"',
  )

  // StudioView chat composer integration
  assert.ok(
    !studioSource.includes('to="/settings/memory?returnTo=/chat"'),
    'Studio keeps preference management in Settings',
  )
})

test('4. Localization: Indonesian dictionary contains all Memory & Taste strings', () => {
  const localeSource = fs.readFileSync(idLocalePath, 'utf8')
  const dictionary = JSON.parse(localeSource)

  assert.equal(
    dictionary['Agent Memory'],
    'Memori Agen',
    'Agent Memory must be translated to Memori Agen',
  )
  assert.equal(
    dictionary['Agent Memory / Taste'],
    'Memori & Kebiasaan Agen',
    'Agent Memory / Taste must be translated to Memori & Kebiasaan Agen',
  )
  assert.equal(
    dictionary['Brand/Product Taste'],
    'Gaya Penulisan & Merek',
    'Brand/Product Taste must be translated to Gaya Penulisan & Merek',
  )
  assert.equal(
    dictionary['Technical Taste'],
    'Aturan Kerja & Konvensi Teknis',
    'Technical Taste must be translated to Aturan Kerja & Konvensi Teknis',
  )
  assert.equal(
    dictionary['Lupakan poin ini'],
    'Lupakan poin ini',
    'Lupakan poin ini must be present in Indonesian dictionary',
  )
  assert.equal(
    dictionary['Reset Semua Memori Agen?'],
    'Reset Semua Memori Agen?',
    'Reset Semua Memori Agen? must be present in dictionary',
  )
  assert.equal(
    dictionary['Add Preference'],
    'Tambah Preferensi',
    'Add Preference must be translated to Tambah Preferensi',
  )
  assert.equal(
    dictionary['Reset All Memory'],
    'Reset Semua Memori',
    'Reset All Memory must be translated to Reset Semua Memori',
  )
  assert.ok(
    dictionary['This will reset your agent preferences back to the standard Cuan templates. Any custom preferences you added will be removed.'],
    'Reset explanation must be translated',
  )
})
