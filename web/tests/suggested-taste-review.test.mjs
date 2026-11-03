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
const studioViewPath = path.join(
  __dirname,
  '..',
  'src',
  'features',
  'inbox',
  'StudioView.tsx',
)
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. API Layer: Candidate Taste & Reflection API client contracts exist', () => {
  assert.ok(fs.existsSync(tasteApiPath), 'src/api/taste.ts must exist')
  const source = fs.readFileSync(tasteApiPath, 'utf8')

  // Data contracts
  assert.ok(
    source.includes('export interface TasteCandidate'),
    'Must export TasteCandidate interface',
  )
  assert.ok(
    source.includes('export interface TasteCandidatesResponse'),
    'Must export TasteCandidatesResponse interface',
  )
  assert.ok(
    source.includes('export interface ReflectTasteRequest'),
    'Must export ReflectTasteRequest interface',
  )
  assert.ok(
    source.includes('export interface ReflectTasteResponse'),
    'Must export ReflectTasteResponse interface',
  )
  assert.ok(
    source.includes('export interface AcceptCandidateRequest'),
    'Must export AcceptCandidateRequest interface',
  )
  assert.ok(
    source.includes('export interface AcceptCandidateResponse'),
    'Must export AcceptCandidateResponse interface',
  )
  assert.ok(
    source.includes('export interface DismissCandidateResponse'),
    'Must export DismissCandidateResponse interface',
  )

  // API Client endpoints
  assert.ok(
    source.includes('getCandidates: async') &&
      source.includes('/api/v1/taste/candidates'),
    'tasteApi must implement getCandidates (/api/v1/taste/candidates)',
  )
  assert.ok(
    source.includes('reflectTaste: async') &&
      source.includes('/api/v1/taste/reflect'),
    'tasteApi must implement reflectTaste (/api/v1/taste/reflect)',
  )
  assert.ok(
    source.includes('acceptCandidate: async') &&
      source.includes('/api/v1/taste/candidates/'),
    'tasteApi must implement acceptCandidate (/api/v1/taste/candidates/:id/accept)',
  )
  assert.ok(
    source.includes('dismissCandidate: async') &&
      source.includes('/api/v1/taste/candidates/'),
    'tasteApi must implement dismissCandidate (/api/v1/taste/candidates/:id/dismiss)',
  )
})

test('2. UI Component: AgentMemoryView Suggested Taste tab, counter badge, and reflection controls', () => {
  assert.ok(
    fs.existsSync(agentMemoryViewPath),
    'AgentMemoryView.tsx must exist',
  )
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  // View mode switcher: Active Memory vs Suggested Taste
  assert.ok(
    source.includes('activeView') &&
      source.includes('setActiveView'),
    'Must maintain activeView state toggle between active and suggested',
  )
  assert.ok(
    source.includes('tab') && (source.includes('suggested') || source.includes('pending')),
    'Must support query parameter ?tab=suggested or ?view=suggested for deep linking',
  )

  // Sub-navigation tabs & counter badge
  assert.ok(
    source.includes('Suggested Taste / Pending Reflection') ||
      source.includes('Suggested Taste'),
    'Must render Suggested Taste / Pending Reflection tab',
  )
  assert.ok(
    source.includes('suggested-tab-counter'),
    'Must render data-testid="suggested-tab-counter" for candidate badge',
  )

  // Autonomous Reflection trigger
  assert.ok(
    source.includes('Reflect Now') || source.includes('Refleksi Sekarang'),
    'Must offer Reflect Now action button',
  )
  assert.ok(
    source.includes('handleTriggerReflection'),
    'Must implement handleTriggerReflection calling tasteApi.reflectTaste',
  )
})

test('3. Review Controls: Human-in-the-Loop Accept, Edit & Save, and Dismiss controls', () => {
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  // Candidate card layout & rationale display
  assert.ok(
    source.includes('candidate-card-') || source.includes('filteredCandidates.map'),
    'Must render candidate cards with dynamic id mapping',
  )
  assert.ok(
    source.includes('Rationale') || source.includes('reason') || source.includes('source_snippet'),
    'Must display rationale or source context explaining where preference was found',
  )

  // Review Controls
  assert.ok(
    source.includes('handleAcceptCandidate'),
    'Must implement 1-click handleAcceptCandidate',
  )
  assert.ok(
    source.includes('handleDismissCandidate'),
    'Must implement 1-click handleDismissCandidate',
  )
  assert.ok(
    source.includes('handleOpenEditCandidate'),
    'Must implement handleOpenEditCandidate to open edit modal before approval',
  )

  // Touch accessibility (min-h-11 >= 44px)
  assert.ok(
    source.includes('min-h-11'),
    'Must respect touch target standard (min-h-11)',
  )
})

// Studio has no preference widgets; memory remains available in Settings.

test('5. Localization: Indonesian dictionary contains all Suggested Taste & Review strings', () => {
  const localeSource = fs.readFileSync(idLocalePath, 'utf8')
  const dictionary = JSON.parse(localeSource)

  assert.ok(
    dictionary['Suggested Taste / Pending Reflection'],
    'Must translate Suggested Taste / Pending Reflection',
  )
  assert.ok(
    dictionary['Suggested Taste'],
    'Must translate Suggested Taste',
  )
  assert.ok(
    dictionary['Pending Reflection'],
    'Must translate Pending Reflection',
  )
  assert.ok(
    dictionary['Suggested Preferences'],
    'Must translate Suggested Preferences',
  )
  assert.ok(
    dictionary['Active Memory'],
    'Must translate Active Memory',
  )
  assert.ok(
    dictionary['Reflect Now'],
    'Must translate Reflect Now',
  )
  assert.ok(
    dictionary['Review Suggestions'],
    'Must translate Review Suggestions',
  )
  assert.ok(
    dictionary['Terima'],
    'Must translate Terima',
  )
  assert.ok(
    dictionary['Edit & Save'],
    'Must translate Edit & Save',
  )
  assert.ok(
    dictionary['Abaikan'],
    'Must translate Abaikan',
  )
  assert.ok(
    dictionary['Rationale'],
    'Must translate Rationale',
  )
  assert.ok(
    dictionary['Candidate preference approved and promoted to active memory.'],
    'Must translate Candidate preference approved',
  )
  assert.ok(
    dictionary['Candidate preference dismissed.'],
    'Must translate Candidate preference dismissed',
  )
  assert.ok(
    dictionary['No suggested preferences pending'],
    'Must translate No suggested preferences pending',
  )
})
