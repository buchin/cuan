import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const editAgentModalPath = path.join(__dirname, '..', 'src', 'features', 'team', 'EditAgentModal.tsx')
const teamViewPath = path.join(__dirname, '..', 'src', 'features', 'team', 'TeamView.tsx')
const agentsApiPath = path.join(__dirname, '..', 'src', 'api', 'agents.ts')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. API Layer: Agent interface and agentsApi model config methods exist', () => {
  const source = fs.readFileSync(agentsApiPath, 'utf8')

  assert.ok(
    source.includes('fallback_model_id?: string'),
    'Agent interface must include optional fallback_model_id'
  )
  assert.ok(
    source.includes('temperature?: number'),
    'Agent interface must include optional temperature'
  )
  assert.ok(
    source.includes('token_budget?: number'),
    'Agent interface must include optional token_budget'
  )
  assert.ok(
    source.includes('interface AgentModelConfig'),
    'Must define AgentModelConfig interface'
  )
  assert.ok(
    source.includes('interface UpdateAgentModelConfigRequest'),
    'Must define UpdateAgentModelConfigRequest interface'
  )
  assert.ok(
    source.includes('getModelConfig: async (id: string)'),
    'agentsApi must expose getModelConfig endpoint helper'
  )
  assert.ok(
    source.includes('updateModelConfig: async ('),
    'agentsApi must expose updateModelConfig endpoint helper'
  )
})

test('2. EditAgentModal component architecture and touch target standards (>= 44x44px)', () => {
  assert.ok(fs.existsSync(editAgentModalPath), 'EditAgentModal.tsx component must exist')
  const source = fs.readFileSync(editAgentModalPath, 'utf8')

  assert.ok(
    source.includes('export function EditAgentModal'),
    'Must export EditAgentModal component'
  )
  assert.ok(
    source.includes('role="dialog"') && source.includes('aria-modal="true"'),
    'Must be accessible dialog modal with aria-modal="true"'
  )
  assert.ok(
    source.includes('min-h-11 min-w-11') || source.includes('min-h-11'),
    'Must satisfy minimum 44px (min-h-11) mobile touch target standard'
  )
})

test('3. AI Model Selector, Fallback Routing, and Hyperparameters', () => {
  const source = fs.readFileSync(editAgentModalPath, 'utf8')

  // Primary model selector
  assert.ok(
    source.includes('id="agent-primary-model"'),
    'Must provide primary AI model select input'
  )

  // Fallback model selector
  assert.ok(
    source.includes('id="agent-fallback-model"'),
    'Must provide fallback model select input'
  )
  assert.ok(
    source.includes("No fallback model"),
    'Must allow disabling fallback routing'
  )

  // Hyperparameters
  assert.ok(
    source.includes('id="agent-temperature"'),
    'Must provide temperature slider control'
  )
  assert.ok(
    source.includes('id="agent-token-budget"'),
    'Must provide token budget input'
  )
  assert.ok(
    source.includes('Precise (0.2)') && source.includes('Balanced (0.7)') && source.includes('Creative (1.0)'),
    'Must provide temperature quick preset chips'
  )
})

test('4. System Prompt, Presets, and Form Validations', () => {
  const source = fs.readFileSync(editAgentModalPath, 'utf8')

  assert.ok(
    source.includes('id="agent-system-prompt"'),
    'Must provide system prompt textarea'
  )
  assert.ok(
    source.includes('PROMPT_PRESETS'),
    'Must provide prompt preset configurations'
  )
  assert.ok(
    source.includes('AI model cannot be empty'),
    'Must validate required model_id'
  )
  assert.ok(
    source.includes('temperature < 0.0 || temperature > 2.0'),
    'Must validate temperature range [0.0, 2.0]'
  )
  assert.ok(
    source.includes('parsedBudget < 0'),
    'Must validate non-negative token budget'
  )
})

test('5. TeamView integration: Direct pencil edit button and AgentActions menu option', () => {
  const source = fs.readFileSync(teamViewPath, 'utf8')

  assert.ok(
    source.includes('Pencil'),
    'TeamView must import Pencil icon from lucide-react'
  )
  assert.ok(
    source.includes('EditAgentModal'),
    'TeamView must import and render EditAgentModal'
  )
  assert.ok(
    source.includes('<Pencil className="h-4 w-4" />'),
    'TeamView must render Pencil icon'
  )
  assert.ok(
    source.includes('onClick={() => setEditingAgent(a)}'),
    'Agent cards must provide trigger to open EditAgentModal'
  )
  assert.ok(
    source.includes('onEdit?: () => void'),
    'AgentActions must support onEdit callback'
  )
  assert.ok(
    source.includes('Fallback:'),
    'Agent card must display Fallback model chip when configured'
  )
})

test('6. Indonesian Locale Support for Model Configuration and Edit Agent UI', () => {
  const rawLocale = fs.readFileSync(idLocalePath, 'utf8')
  const locale = JSON.parse(rawLocale)

  assert.ok(locale['Configure Agent'], 'Must translate Configure Agent')
  assert.ok(locale['Edit Config'], 'Must translate Edit Config')
  assert.ok(locale['Primary AI Model'], 'Must translate Primary AI Model')
  assert.ok(locale['Fallback Model (Auto-Routing)'], 'Must translate Fallback Model')
  assert.ok(locale['Temperature'], 'Must translate Temperature')
  assert.ok(locale['Token Budget (Max Output Tokens)'], 'Must translate Token Budget')
  assert.ok(locale['Save Configuration'], 'Must translate Save Configuration')
  assert.ok(locale['AI model cannot be empty'], 'Must translate AI model validation')
})
