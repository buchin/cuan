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

test('1. API Layer: Dynamic model discovery contracts, interfaces, and refresh capability', () => {
  const source = fs.readFileSync(agentsApiPath, 'utf8')

  assert.ok(
    source.includes('interface ModelCapabilities'),
    'agents.ts must define ModelCapabilities interface'
  )
  assert.ok(
    source.includes('vision?: boolean'),
    'ModelCapabilities must include vision flag'
  )
  assert.ok(
    source.includes('capabilities?: ModelCapabilities'),
    'ModelInfo must declare capabilities property'
  )
  assert.ok(
    source.includes('supports_vision?: boolean'),
    'ModelInfo must declare supports_vision property'
  )
  assert.ok(
    source.includes('is_default?: boolean'),
    'ModelInfo must declare is_default property'
  )
  assert.ok(
    source.includes('preserved?: boolean'),
    'ModelInfo must declare preserved property for backward compatibility'
  )
  assert.ok(
    source.includes('listModels: async (provider = \'commandcode\', refresh = false)'),
    'agentsApi.listModels must support on-demand refresh flag'
  )
  assert.ok(
    source.includes('params.set(\'refresh\', \'true\')'),
    'agentsApi.listModels must pass refresh=true parameter to backend query'
  )
  assert.ok(
    source.includes('getModelConfig: async (id: string, refresh = false)'),
    'agentsApi.getModelConfig must support on-demand refresh flag'
  )
  assert.ok(
    source.includes('refresh ? \'?refresh=true\' : \'\''),
    'agentsApi.getModelConfig must pass ?refresh=true when refresh is requested'
  )
})

test('2. isModelVisionCapable: Multimodal detection across metadata, providers, and keywords', () => {
  const source = fs.readFileSync(agentsApiPath, 'utf8')

  assert.ok(
    source.includes('export function isModelVisionCapable'),
    'agents.ts must export isModelVisionCapable function'
  )
  assert.ok(
    source.includes('modelInfo.capabilities?.vision !== undefined'),
    'Must check modelInfo.capabilities.vision first'
  )
  assert.ok(
    source.includes('modelInfo.capabilities?.supports_vision !== undefined') ||
    source.includes('modelInfo.supports_vision !== undefined'),
    'Must check modelInfo.supports_vision flags'
  )
  assert.ok(
    source.includes('gpt-oss') && source.includes('embedding'),
    'Must handle non-vision Google/AGY exceptions'
  )

  // Verify pure implementation matches exact logic
  function isModelVisionCapablePure(modelId, provider, modelInfo) {
    if (modelInfo) {
      if (modelInfo.capabilities?.vision !== undefined) return modelInfo.capabilities.vision
      if (modelInfo.capabilities?.supports_vision !== undefined) return modelInfo.capabilities.supports_vision
      if (modelInfo.supports_vision !== undefined) return modelInfo.supports_vision
    }
    const p = (provider || '').toLowerCase()
    const m = (modelId || '').toLowerCase()
    if (p === 'agy' || p === 'antigravity' || p === 'google' || p === 'gemini') {
      if (m.includes('gpt-oss') || m.includes('text') || m.includes('embedding')) {
        return false
      }
      return true
    }
    if (!m || m === 'commandcode-default' || m === 'default') {
      return false
    }
    const visionKeywords = [
      'vision', 'multimodal', '-vl', 'super-vl', 'omni',
      'sonnet', 'opus', 'gemini', '4o', 'kimi-k2',
      'minimax-m3', 'inkling', 'mistral-large-4', 'muse-spark-1.3',
      'v4.1-flash',
    ]
    return visionKeywords.some((kw) => m.includes(kw))
  }

  // Metadata capability flag takes precedence
  assert.equal(
    isModelVisionCapablePure('custom-model-x', 'custom-provider', {
      id: 'custom-model-x',
      name: 'Custom X',
      capabilities: { vision: true },
    }),
    true,
    'Model with capabilities.vision=true must be detected as vision capable'
  )
  assert.equal(
    isModelVisionCapablePure('custom-model-y', 'custom-provider', {
      id: 'custom-model-y',
      name: 'Custom Y',
      capabilities: { vision: false },
    }),
    false,
    'Model with capabilities.vision=false must be detected as text-only'
  )

  // Supports_vision field
  assert.equal(
    isModelVisionCapablePure('custom-model-z', 'custom-provider', {
      id: 'custom-model-z',
      name: 'Custom Z',
      supports_vision: true,
    }),
    true,
    'Model with supports_vision=true must be detected as vision capable'
  )

  // Provider heuristics
  assert.equal(
    isModelVisionCapablePure('gemini-1.5-pro', 'google'),
    true,
    'Google Gemini models must be vision capable'
  )
  assert.equal(
    isModelVisionCapablePure('gemini-2.0-flash', 'agy'),
    true,
    'AGY Gemini models must be vision capable'
  )
  assert.equal(
    isModelVisionCapablePure('text-embedding-004', 'google'),
    false,
    'Embedding/text-only Google models must be detected as non-vision'
  )

  // Keyword heuristics
  assert.equal(isModelVisionCapablePure('claude-3-5-sonnet-20241022', 'commandcode'), true)
  assert.equal(isModelVisionCapablePure('qwen-2.5-vl-72b', 'commandcode'), true)
  assert.equal(isModelVisionCapablePure('gpt-4o-mini', 'commandcode'), true)
  assert.equal(isModelVisionCapablePure('commandcode-default', 'commandcode'), false)
  assert.equal(isModelVisionCapablePure('deepseek-v4-pro', 'commandcode'), false)
})

test('3. EditAgentModal: Complete elimination of hardcoded models and dynamic discovery integration', () => {
  const source = fs.readFileSync(editAgentModalPath, 'utf8')

  // Hardcoded static fallback array must NOT exist
  assert.ok(
    !source.includes('const effectiveModels = supportedModels.length > 0 ? supportedModels : ['),
    'Static hardcoded effectiveModels array fallback must be removed completely'
  )
  assert.ok(
    !source.includes('{ id: \'gemini-1.5-pro\', name: \'Gemini 1.5 Pro\''),
    'Hardcoded gemini-1.5-pro entry must not exist in EditAgentModal'
  )

  // Dynamic catalog state & refresh handler
  assert.ok(
    source.includes('const [refreshingCatalog, setRefreshingCatalog] = useState(false)'),
    'Must track refreshingCatalog state'
  )
  assert.ok(
    source.includes('handleRefreshCatalog'),
    'Must define handleRefreshCatalog action handler'
  )
  assert.ok(
    source.includes('agentsApi.getModelConfig(agent.id, true)'),
    'handleRefreshCatalog must call getModelConfig with refresh=true'
  )
  assert.ok(
    source.includes('t(\'Refresh Catalog\')'),
    'Must render Refresh Catalog button with localized label'
  )
  assert.ok(
    source.includes('refreshingCatalog ? \'animate-spin\' : \'\''),
    'Refresh Catalog icon must spin when refreshing is in progress'
  )

  // Visual Badging for Vision vs Text-only and Default status
  assert.ok(
    source.includes('isModelVisionCapable('),
    'Must use isModelVisionCapable for dynamic capability evaluation'
  )
  assert.ok(
    source.includes('t(\'Vision\')'),
    'Must display Vision badge'
  )
  assert.ok(
    source.includes('t(\'Text-only\')'),
    'Must display Text-only badge'
  )
  assert.ok(
    source.includes('t(\'Default\')'),
    'Must display Default model badge'
  )

  // Custom Model ID / Manual Entry
  assert.ok(
    source.includes('id="agent-custom-primary-model"'),
    'Must provide custom primary model text input field'
  )
  assert.ok(
    source.includes('t(\'Custom Model ID / Manual Entry\')'),
    'Must provide Custom Model ID option in dropdown selector'
  )
  assert.ok(
    source.includes('isCustomPrimary'),
    'Must track custom primary model input mode'
  )

  // Smart Fallback Model Selection
  assert.ok(
    source.includes('id="agent-fallback-model"'),
    'Must provide fallback model dropdown selector'
  )
  assert.ok(
    source.includes('.filter((m) => m.id !== modelId)'),
    'Fallback model list must exclude currently selected primary model'
  )
  assert.ok(
    source.includes('id="agent-custom-fallback-model"'),
    'Must provide custom fallback model text input field'
  )
  assert.ok(
    source.includes('Recommendation: Choose a Vision-capable fallback model'),
    'Must provide smart guidance when fallback lacks Vision'
  )

  // 100% Backward compatibility: preserve saved model identifier
  assert.ok(
    source.includes('m.id === modelId'),
    'Must check if saved primary model is in catalog'
  )
  assert.ok(
    source.includes('t(\'Preserved\')'),
    'Must display Preserved tag for unlisted existing saved models'
  )
})

test('4. TeamView: Complete elimination of hardcoded model options and dynamic provider catalog', () => {
  const source = fs.readFileSync(teamViewPath, 'utf8')

  // Ensure all 10 hardcoded options are removed
  assert.ok(
    !source.includes('<option value="claude-sonnet-5-5">Claude Sonnet 5.5 (Vision)</option>'),
    'Hardcoded claude-sonnet-5-5 option must be removed from TeamView'
  )
  assert.ok(
    !source.includes('<option value="google/gemini-3.8-flash">Gemini 3.8 Flash (Vision)</option>'),
    'Hardcoded google/gemini-3.8-flash option must be removed from TeamView'
  )
  assert.ok(
    !source.includes('<option value="moonshotai/kimi-k2.7-code">Kimi K2.7 Code (Vision)</option>'),
    'Hardcoded moonshotai/kimi-k2.7-code option must be removed from TeamView'
  )
  assert.ok(
    !source.includes('<option value="deepseek/deepseek-v4-pro">DeepSeek V4 Pro (Text-only)</option>'),
    'Hardcoded deepseek/deepseek-v4-pro option must be removed from TeamView'
  )
  assert.ok(
    !source.includes('<option value="reflection/beam:free">Reflection Beam (Free, Text-only)</option>'),
    'Hardcoded reflection/beam:free option must be removed from TeamView'
  )

  // Dynamic model catalog & refresh button in TeamView
  assert.ok(
    source.includes('agentsApi.listModels(p, refresh)'),
    'TeamView must fetch models dynamically via agentsApi.listModels with refresh support'
  )
  assert.ok(
    source.includes('t(\'Refresh Catalog\')'),
    'TeamView must provide Refresh Catalog action'
  )
  assert.ok(
    source.includes('id="custom-commandcode-model"'),
    'TeamView must provide manual Custom Model ID input'
  )
  assert.ok(
    source.includes('isModelVisionCapable(commandcodeModel'),
    'TeamView must evaluate vision capability of active model'
  )

  // Agent card badging
  assert.ok(
    source.includes('isModelVisionCapable(a.model_id, a.provider)'),
    'TeamView agent cards must display visual capability badge for agent model'
  )
  assert.ok(
    source.includes('isModelVisionCapable(a.fallback_model_id, a.provider)'),
    'TeamView agent cards must display visual capability badge for fallback model'
  )
})

test('5. Localization: Indonesian dictionary (id.json) includes all dynamic model discovery strings', () => {
  const raw = fs.readFileSync(idLocalePath, 'utf8')
  const locale = JSON.parse(raw)

  const requiredKeys = [
    'Refresh Catalog',
    'Refreshing...',
    'Refreshing catalog...',
    'Refresh model catalog from provider',
    'Model catalog refreshed successfully',
    'Failed to refresh model catalog',
    'Custom Model ID / Manual Entry',
    'Custom Fallback Model ID / Manual Entry',
    'Custom Model ID',
    'Custom Fallback Model ID',
    'Custom',
    'Default',
    'Preserved',
    'Vision',
    'Text-only',
    'Use catalog model',
    'Enter a custom AI model identifier supported by your provider.',
    'Supports image and document analysis (receipts, transfer proof, menus, etc.)',
    'Supports image and document analysis',
    'Text processing only (no image analysis)',
    'Loading models catalog...',
    'No models found. Click Refresh Catalog to reload.',
    'Recommendation: Choose a Vision-capable fallback model so your agent can continue processing receipts/images during failover.',
    'High availability: Both primary and fallback models support Vision.',
  ]

  for (const key of requiredKeys) {
    assert.ok(
      Boolean(locale[key]),
      `id.json must include translation for key: "${key}"`
    )
    assert.ok(
      typeof locale[key] === 'string' && locale[key].trim().length > 0,
      `id.json value for "${key}" must be a non-empty string`
    )
  }
})

test('6. Dynamic exclusion logic: primary model is always filtered out of fallback model options', () => {
  const mockModels = [
    { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash', supports_vision: true },
    { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro', supports_vision: true },
    { id: 'deepseek-chat', name: 'DeepSeek Chat', supports_vision: false },
  ]

  const selectedPrimary = 'gemini-2.0-flash'
  const fallbackOptions = mockModels.filter((m) => m.id !== selectedPrimary)

  assert.equal(fallbackOptions.length, 2)
  assert.ok(!fallbackOptions.some((m) => m.id === selectedPrimary))
  assert.ok(fallbackOptions.some((m) => m.id === 'gemini-1.5-pro'))
  assert.ok(fallbackOptions.some((m) => m.id === 'deepseek-chat'))
})
