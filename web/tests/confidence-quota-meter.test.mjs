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

test('1. API Layer: Taste Quota, Config, Undo, and Confidence contracts exist', () => {
  assert.ok(fs.existsSync(tasteApiPath), 'src/api/taste.ts must exist')
  const source = fs.readFileSync(tasteApiPath, 'utf8')

  // Quota & Config Data contracts
  assert.ok(
    source.includes('export interface TasteQuotaCategory'),
    'Must export TasteQuotaCategory interface',
  )
  assert.ok(
    source.includes('export interface TasteQuotaResponse'),
    'Must export TasteQuotaResponse interface',
  )
  assert.ok(
    source.includes('export interface TasteConfig'),
    'Must export TasteConfig interface',
  )
  assert.ok(
    source.includes('export interface UpdateTasteConfigRequest'),
    'Must export UpdateTasteConfigRequest interface',
  )
  assert.ok(
    source.includes('export interface UndoCandidateResponse'),
    'Must export UndoCandidateResponse interface',
  )

  // CandidateTaste alias & Confidence field
  assert.ok(
    source.includes('export type CandidateTaste = TasteCandidate'),
    'Must export CandidateTaste type alias for TasteCandidate',
  )
  assert.ok(
    source.includes('confidence?: number') || source.includes('confidence: number'),
    'TasteCandidate must contain confidence field (0.0 - 1.0)',
  )

  // TasteItem auto_approved metadata
  assert.ok(
    source.includes('auto_approved?: boolean') || source.includes('auto_approved: boolean'),
    'TasteItem must contain auto_approved metadata',
  )

  // ReflectTasteResponse auto_approved list
  assert.ok(
    source.includes('auto_approved?: TasteCandidate[]'),
    'ReflectTasteResponse must contain auto_approved candidates array',
  )

  // API Client endpoints
  assert.ok(
    source.includes('getQuota: async') && source.includes('/api/v1/taste/quota'),
    'tasteApi must implement getQuota (/api/v1/taste/quota)',
  )
  assert.ok(
    source.includes('getConfig: async') && source.includes('/api/v1/taste/config'),
    'tasteApi must implement getConfig (/api/v1/taste/config)',
  )
  assert.ok(
    source.includes('updateConfig: async') &&
      source.includes('/api/v1/taste/config') &&
      source.includes("'PUT'"),
    'tasteApi must implement updateConfig (/api/v1/taste/config via PUT)',
  )
  assert.ok(
    source.includes('undoCandidate: async') &&
      source.includes('/api/v1/taste/candidates/') &&
      source.includes('/undo'),
    'tasteApi must implement undoCandidate (/api/v1/taste/candidates/:id/undo via POST)',
  )
})

test('2. UI Component: Hermes-Style Memory Quota Meter in AgentMemoryView', () => {
  assert.ok(fs.existsSync(agentMemoryViewPath), 'AgentMemoryView.tsx must exist')
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  // Quota state and loading hook
  assert.ok(
    source.includes('tasteApi.getQuota') && source.includes('setQuota'),
    'AgentMemoryView must fetch and maintain quota state via tasteApi.getQuota',
  )

  // Hermes Quota Meter Section
  assert.ok(
    source.includes('data-testid="hermes-quota-meter"'),
    'Must render hermes-quota-meter section container',
  )

  // Brand Taste & Technical Taste Quota Meters
  assert.ok(
    source.includes('data-testid="quota-meter-brand"'),
    'Must render Brand Taste quota meter card',
  )
  assert.ok(
    source.includes('data-testid="quota-meter-technical"'),
    'Must render Technical Taste quota meter card',
  )

  // Quota Count Indicators (e.g. '8/15 Butir Terpakai')
  assert.ok(
    source.includes('data-testid="quota-count-brand"'),
    'Must render quota-count-brand indicator',
  )
  assert.ok(
    source.includes('data-testid="quota-count-technical"'),
    'Must render quota-count-technical indicator',
  )
  assert.ok(
    source.includes('Butir Terpakai') || source.includes('points used'),
    'Must display capacity usage text with Butir Terpakai',
  )

  // Visual Progress Bars
  assert.ok(
    source.includes('data-testid="quota-bar-brand"'),
    'Must render progress bar fill for brand taste',
  )
  assert.ok(
    source.includes('data-testid="quota-bar-technical"'),
    'Must render progress bar fill for technical taste',
  )

  // Visual Warnings: >= 12 (Approaching Limit) and 15 (Quota Full)
  assert.ok(
    source.includes('brandWarning') || source.includes('brandUsed >= 12'),
    'Must compute visual warning when brand items >= 12',
  )
  assert.ok(
    source.includes('brandFull') || source.includes('brandUsed >= 15'),
    'Must compute full quota condition when brand items >= 15',
  )
  assert.ok(
    source.includes('bg-rose-500'),
    'Must style progress bar with danger color (rose-500) when quota is full',
  )
  assert.ok(
    source.includes('bg-amber-500'),
    'Must style progress bar with warning color (amber-500) when approaching limit (>= 12)',
  )
})

test('3. UI Component: Confidence Badges on Candidate Taste Cards', () => {
  assert.ok(fs.existsSync(agentMemoryViewPath), 'AgentMemoryView.tsx must exist')
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  // Confidence badge rendering
  assert.ok(
    source.includes('data-testid={`confidence-badge-${candidate.id}`}') ||
      source.includes('data-testid={"confidence-badge-" + candidate.id}'),
    'Must render testable confidence badge on candidate taste card',
  )

  // Keyakinan percentage calculation
  assert.ok(
    source.includes('candidate.confidence') &&
      source.includes('Math.round('),
    'Must calculate confidence percentage from candidate.confidence float',
  )

  // Status colors: green (>= 80%) vs amber/orange (< 80%)
  assert.ok(
    source.includes('bg-emerald-500') && source.includes('bg-amber-500'),
    'Must color code confidence badges with emerald for high confidence (>= 80%) and amber for moderate (< 80%)',
  )
})

test('4. UI Component: Smart Auto-Approval Controls & 1-Click Undo Notification', () => {
  assert.ok(fs.existsSync(agentMemoryViewPath), 'AgentMemoryView.tsx must exist')
  const source = fs.readFileSync(agentMemoryViewPath, 'utf8')

  // Controls section & Toggle Switch
  assert.ok(
    source.includes('data-testid="auto-approve-controls"'),
    'Must render auto-approve-controls section',
  )
  assert.ok(
    source.includes('data-testid="auto-approve-toggle"'),
    'Must render auto-approve-toggle switch button',
  )
  assert.ok(
    source.includes('handleToggleAutoApprove'),
    'Must implement toggle handler updating taste config',
  )

  // Threshold slider / input
  assert.ok(
    source.includes('data-testid="auto-approve-threshold-slider"'),
    'Must render threshold range slider',
  )
  assert.ok(
    source.includes('handleUpdateThreshold'),
    'Must implement threshold update handler calling tasteApi.updateConfig',
  )

  // Toast Notification with 1-Click Undo
  assert.ok(
    source.includes('data-testid="auto-approve-toast"'),
    'Must render auto-approve-toast notification container',
  )
  assert.ok(
    source.includes('data-testid="undo-auto-approve-btn"'),
    'Must render 1-click Undo button inside auto-approve toast',
  )
  assert.ok(
    source.includes('tasteApi.undoCandidate'),
    'Must invoke tasteApi.undoCandidate when clicking undo',
  )

  // Active Memory Card Auto-Approved Indicator
  assert.ok(
    source.includes('data-testid={`auto-approved-badge-${item.id}`}') ||
      source.includes('auto-approved-badge'),
    'Must render auto-approved badge on active memory cards for promoted items',
  )
})

// Studio has no preference widgets; memory remains available in Settings.

test('6. Localization: Indonesian dictionary contains all Quota, Confidence, and Auto-Approve strings', () => {
  assert.ok(fs.existsSync(idLocalePath), 'src/locales/id.json must exist')
  const locales = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  // Quota Meter strings
  assert.ok(locales['Hermes Memory Quota Meter'], 'Must translate Hermes Memory Quota Meter')
  assert.ok(locales['Memory Quota (Hermes)'] || locales['Kapasitas Memori (Hermes)'], 'Must translate Memory Quota (Hermes)')
  assert.ok(locales['Butir Terpakai'], 'Must translate Butir Terpakai')
  assert.ok(locales['Quota Full'], 'Must translate Quota Full')
  assert.ok(locales['Approaching Limit'], 'Must translate Approaching Limit')
  assert.ok(locales['Optimal Capacity'] || locales['Optimal'], 'Must translate Optimal Capacity')

  // Confidence Badge strings
  assert.ok(locales['Keyakinan'], 'Must translate Keyakinan')
  assert.ok(locales['Confidence'], 'Must translate Confidence')

  // Auto-Approve Controls & Undo strings
  assert.ok(locales['Smart Auto-Approval'], 'Must translate Smart Auto-Approval')
  assert.ok(locales['Auto-Approve Enabled'], 'Must translate Auto-Approve Enabled')
  assert.ok(locales['Confidence Threshold'] || locales['Ambang Batas Keyakinan'], 'Must translate Confidence Threshold')
  assert.ok(locales['Preferensi baru otomatis disetujui'], 'Must translate Preferensi baru otomatis disetujui')
  assert.ok(locales['Batalkan'] || locales['Undo'], 'Must translate Batalkan (Undo)')
  assert.ok(locales['Auto-approved'] || locales['Otomatis Disetujui'], 'Must translate Auto-approved')
  assert.ok(locales['Auto-approval undone, preference returned to suggested list.'], 'Must translate Undo confirmation message')
})
