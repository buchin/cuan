import { useEffect, useRef, useState } from 'react'
import {
  X,
  Bot,
  Sparkles,
  Sliders,
  Cpu,
  ShieldCheck,
  Check,
  AlertCircle,
  RefreshCw,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import {
  agentsApi,
  type Agent,
  type AgentModelConfig,
  type ModelInfo,
} from '../../api/agents'

export interface EditAgentModalProps {
  agent: Agent
  open: boolean
  onClose: () => void
  onSaved: (updatedAgent: Agent) => void
}

interface PromptPreset {
  label: string
  prompt: string
}

const PROMPT_PRESETS: PromptPreset[] = [
  {
    label: 'Scrum Master (@sm)',
    prompt:
      'You are @sm, the Scrum Master and Work Coordinator. Lead refinement, assign work clearly, coordinate team specialists, verify acceptance criteria, and enforce definition of done.',
  },
  {
    label: 'Senior Developer (@dev)',
    prompt:
      'You are @dev, the Senior Software Engineer. Implement clean, modular, tested, and reliable code according to project conventions, ensuring zero regressions and safe deployments.',
  },
  {
    label: 'Customer Support (@support)',
    prompt:
      'You are @support, the Customer Experience & Support Specialist. Assist users promptly with empathy, clarity, step-by-step guidance, and escalate operational issues accurately.',
  },
  {
    label: 'Precise Code Reviewer',
    prompt:
      'You are a rigorous code reviewer. Analyze logic, security vulnerabilities, edge cases, type safety, performance, and compliance with architectural principles.',
  },
]

export function EditAgentModal({
  agent,
  open,
  onClose,
  onSaved,
}: EditAgentModalProps) {
  const { t } = useLanguage()
  const modalRef = useRef<HTMLDivElement>(null)

  const [loadingConfig, setLoadingConfig] = useState(false)
  const [saving, setSaving] = useState(false)
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Form State
  const [modelId, setModelId] = useState<string>(agent.model_id || 'gemini-1.5-pro')
  const [fallbackModelId, setFallbackModelId] = useState<string>(agent.fallback_model_id || '')
  const [temperature, setTemperature] = useState<number>(agent.temperature ?? 0.7)
  const [tokenBudget, setTokenBudget] = useState<string>(
    agent.token_budget !== undefined && agent.token_budget !== null ? String(agent.token_budget) : '8192'
  )
  const [systemPrompt, setSystemPrompt] = useState<string>(agent.system_prompt || '')
  const [role, setRole] = useState<string>(agent.role || '')
  const [supportedModels, setSupportedModels] = useState<ModelInfo[]>([])

  // Load config on mount or open
  useEffect(() => {
    if (!open) return
    let active = true

    setFeedback(null)
    setLoadingConfig(true)

    agentsApi
      .getModelConfig(agent.id)
      .then((cfg: AgentModelConfig) => {
        if (!active) return
        if (cfg.model_id) setModelId(cfg.model_id)
        if (cfg.fallback_model_id !== undefined) setFallbackModelId(cfg.fallback_model_id || '')
        if (cfg.temperature !== undefined && cfg.temperature !== null) setTemperature(cfg.temperature)
        if (cfg.token_budget !== undefined && cfg.token_budget !== null) setTokenBudget(String(cfg.token_budget))
        if (cfg.system_prompt !== undefined) setSystemPrompt(cfg.system_prompt || '')
        if (cfg.role) setRole(cfg.role)
        if (cfg.supported_models && cfg.supported_models.length > 0) {
          setSupportedModels(cfg.supported_models)
        }
      })
      .catch(() => {
        // Fallback to agent initial props
        if (!active) return
      })
      .finally(() => {
        if (active) setLoadingConfig(false)
      })

    return () => {
      active = false
    }
  }, [agent.id, open])

  // Keyboard navigation & accessibility focus trap
  useEffect(() => {
    if (!open) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }

      if (e.key === 'Tab' && modalRef.current) {
        const focusable = modalRef.current.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
        )
        if (focusable.length === 0) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]

        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last?.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first?.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  if (!open) return null

  const handleApplyPresetPrompt = (preset: PromptPreset) => {
    setSystemPrompt(preset.prompt)
  }

  const handleApplyTemperaturePreset = (value: number) => {
    setTemperature(value)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setFeedback(null)

    // Form validations
    if (!modelId.trim()) {
      setFeedback({ type: 'error', message: t('AI model cannot be empty') })
      return
    }

    if (temperature < 0.0 || temperature > 2.0 || isNaN(temperature)) {
      setFeedback({ type: 'error', message: t('Temperature must be between 0.0 and 2.0') })
      return
    }

    let parsedBudget: number | undefined
    if (tokenBudget.trim() !== '') {
      parsedBudget = parseInt(tokenBudget.trim(), 10)
      if (isNaN(parsedBudget) || parsedBudget < 0) {
        setFeedback({ type: 'error', message: t('Token budget must be 0 or a positive number') })
        return
      }
    }

    setSaving(true)
    try {
      const res = await agentsApi.updateModelConfig(agent.id, {
        model_id: modelId.trim(),
        fallback_model_id: fallbackModelId.trim(),
        temperature,
        token_budget: parsedBudget,
        system_prompt: systemPrompt.trim(),
        role: role.trim(),
      })

      setFeedback({
        type: 'success',
        message: res.message || t('Agent model configuration saved successfully'),
      })

      if (res.agent) {
        onSaved(res.agent)
      } else {
        onSaved({
          ...agent,
          model_id: modelId.trim(),
          fallback_model_id: fallbackModelId.trim(),
          temperature,
          token_budget: parsedBudget,
          system_prompt: systemPrompt.trim(),
          role: role.trim(),
        })
      }

      // Close modal smoothly after brief feedback
      setTimeout(() => {
        onClose()
      }, 700)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to save agent model configuration'
      setFeedback({ type: 'error', message: msg })
    } finally {
      setSaving(false)
    }
  }

  const effectiveModels = supportedModels.length > 0 ? supportedModels : [
    { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro', display_name: 'Gemini 1.5 Pro' },
    { id: 'gemini-1.5-flash', name: 'Gemini 1.5 Flash', display_name: 'Gemini 1.5 Flash' },
    { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash', display_name: 'Gemini 2.0 Flash' },
  ]

  return (
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 backdrop-blur-xs p-0 sm:p-4 overflow-y-auto"
      onClick={(e) => {
        if (e.target === e.currentTarget && !saving) onClose()
      }}
      role="presentation"
    >
      <div
        ref={modalRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-agent-title"
        className="w-full sm:max-w-xl max-h-[92dvh] sm:max-h-[85dvh] flex flex-col rounded-t-2xl sm:rounded-xl border border-border bg-card text-card-foreground shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-200"
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border/80 px-5 py-3.5 bg-muted/20 shrink-0">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Bot className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 id="edit-agent-title" className="text-base font-semibold leading-tight truncate">
                {t('Configure Agent')} : {agent.name}
              </h2>
              <p className="text-xs text-muted-foreground truncate">
                {t('AI model selection, fallback routing, and prompt behavior')}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={saving}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-border/40 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
            aria-label={t('Close')}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSave} className="flex-1 overflow-y-auto p-5 space-y-5">
          {/* Feedback Banner */}
          {feedback && (
            <div
              className={`flex items-start gap-2.5 p-3 rounded-lg text-xs font-medium border ${
                feedback.type === 'success'
                  ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                  : 'bg-destructive/10 text-destructive border-destructive/20'
              }`}
              role="alert"
            >
              {feedback.type === 'success' ? (
                <Check className="h-4 w-4 shrink-0 mt-0.5" />
              ) : (
                <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              )}
              <span className="flex-1 break-words">{feedback.message}</span>
            </div>
          )}

          {loadingConfig && (
            <div className="flex items-center justify-center gap-2 py-4 text-xs text-muted-foreground">
              <RefreshCw className="h-4 w-4 animate-spin text-primary" />
              <span>{t('Loading agent configuration...')}</span>
            </div>
          )}

          {/* Section 1: AI Model & Fallback Routing */}
          <div className="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              <Cpu className="h-4 w-4 text-primary" />
              <span>{t('Model & Failover Configuration')}</span>
            </div>

            {/* Primary Model */}
            <div className="space-y-1.5">
              <label htmlFor="agent-primary-model" className="block text-xs font-medium text-foreground">
                {t('Primary AI Model')} <span className="text-destructive">*</span>
              </label>
              <select
                id="agent-primary-model"
                value={modelId}
                onChange={(e) => setModelId(e.target.value)}
                disabled={saving}
                className="w-full min-h-11 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50"
              >
                {effectiveModels.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.display_name || m.name || m.id} {m.is_default ? `(${t('Default')})` : ''}
                  </option>
                ))}
                {!effectiveModels.some((m) => m.id === modelId) && (
                  <option value={modelId}>{modelId} ({t('Custom')})</option>
                )}
              </select>
              <p className="text-[11px] text-muted-foreground">
                {t('Default primary model utilized by this agent for tasks and chat generation.')}
              </p>
            </div>

            {/* Fallback Model */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label htmlFor="agent-fallback-model" className="block text-xs font-medium text-foreground">
                  {t('Fallback Model (Auto-Routing)')}
                </label>
                <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
                  <ShieldCheck className="h-3 w-3 text-emerald-500" />
                  {t('High Availability')}
                </span>
              </div>
              <select
                id="agent-fallback-model"
                value={fallbackModelId}
                onChange={(e) => setFallbackModelId(e.target.value)}
                disabled={saving}
                className="w-full min-h-11 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50"
              >
                <option value="">{t('No fallback model (disable auto-routing)')}</option>
                {effectiveModels
                  .filter((m) => m.id !== modelId)
                  .map((m) => (
                    <option key={`fallback-${m.id}`} value={m.id}>
                      {m.display_name || m.name || m.id}
                    </option>
                  ))}
                {fallbackModelId &&
                  fallbackModelId !== modelId &&
                  !effectiveModels.some((m) => m.id === fallbackModelId) && (
                    <option value={fallbackModelId}>{fallbackModelId} ({t('Custom')})</option>
                  )}
              </select>
              <p className="text-[11px] text-muted-foreground">
                {t('Automatically routes queries to this model if the primary model encounters rate limits or quota errors.')}
              </p>
            </div>
          </div>

          {/* Section 2: Generation Hyperparameters */}
          <div className="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              <Sliders className="h-4 w-4 text-primary" />
              <span>{t('Generation Parameters')}</span>
            </div>

            {/* Temperature */}
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs">
                <label htmlFor="agent-temperature" className="font-medium text-foreground">
                  {t('Temperature')}: <span className="font-mono text-primary font-semibold">{temperature.toFixed(2)}</span>
                </label>
                <span className="text-[11px] text-muted-foreground">
                  {temperature <= 0.3
                    ? t('Precise / Deterministic')
                    : temperature <= 0.8
                      ? t('Balanced')
                      : t('Creative')}
                </span>
              </div>
              <input
                id="agent-temperature"
                type="range"
                min="0.0"
                max="2.0"
                step="0.05"
                value={temperature}
                onChange={(e) => setTemperature(parseFloat(e.target.value))}
                disabled={saving}
                className="w-full accent-primary h-2 bg-muted rounded-lg cursor-pointer"
              />
              {/* Presets */}
              <div className="flex flex-wrap gap-1.5 pt-1">
                {[
                  { label: t('Precise (0.2)'), val: 0.2 },
                  { label: t('Balanced (0.7)'), val: 0.7 },
                  { label: t('Creative (1.0)'), val: 1.0 },
                ].map((p) => (
                  <button
                    key={p.val}
                    type="button"
                    onClick={() => handleApplyTemperaturePreset(p.val)}
                    disabled={saving}
                    className={`inline-flex min-h-11 sm:min-h-9 items-center rounded-md px-3 py-1.5 text-xs font-medium border transition-colors ${
                      Math.abs(temperature - p.val) < 0.05
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border/60 bg-background text-muted-foreground hover:bg-muted hover:text-foreground'
                    }`}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Token Budget */}
            <div className="space-y-1.5">
              <label htmlFor="agent-token-budget" className="block text-xs font-medium text-foreground">
                {t('Token Budget (Max Output Tokens)')}
              </label>
              <input
                id="agent-token-budget"
                type="number"
                min="0"
                step="256"
                value={tokenBudget}
                onChange={(e) => setTokenBudget(e.target.value)}
                placeholder="8192"
                disabled={saving}
                className="w-full min-h-11 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50"
              />
              <p className="text-[11px] text-muted-foreground">
                {t('Limits the maximum tokens per completion response. Set 0 or leave empty for default.')}
              </p>
            </div>
          </div>

          {/* Section 3: Agent Persona & Prompting */}
          <div className="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              <Sparkles className="h-4 w-4 text-primary" />
              <span>{t('Persona & System Prompt')}</span>
            </div>

            {/* Role */}
            <div className="space-y-1.5">
              <label htmlFor="agent-role" className="block text-xs font-medium text-foreground">
                {t('Agent Role / Title')}
              </label>
              <input
                id="agent-role"
                type="text"
                value={role}
                onChange={(e) => setRole(e.target.value)}
                placeholder={t('e.g. Scrum Master, Senior Engineer, Support Specialist')}
                disabled={saving}
                className="w-full min-h-11 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50"
              />
            </div>

            {/* System Prompt Presets */}
            <div className="space-y-1.5">
              <label htmlFor="agent-system-prompt" className="block text-xs font-medium text-foreground">
                {t('System Prompt')}
              </label>
              <div className="flex flex-wrap gap-1.5 mb-2">
                {PROMPT_PRESETS.map((preset) => (
                  <button
                    key={preset.label}
                    type="button"
                    onClick={() => handleApplyPresetPrompt(preset)}
                    disabled={saving}
                    className="inline-flex min-h-11 sm:min-h-9 items-center rounded-md border border-border/60 bg-background px-3 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground active:scale-[0.98] transition-colors"
                  >
                    <span>{preset.label}</span>
                  </button>
                ))}
              </div>
              <textarea
                id="agent-system-prompt"
                rows={4}
                value={systemPrompt}
                onChange={(e) => setSystemPrompt(e.target.value)}
                placeholder={t('Enter custom system instructions for this agent...')}
                disabled={saving}
                className="w-full min-h-[110px] rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary disabled:opacity-50 resize-y"
              />
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex flex-col-reverse sm:flex-row items-center justify-end gap-2.5 pt-2 border-t border-border/60">
            <button
              type="button"
              onClick={onClose}
              disabled={saving}
              className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted transition-colors disabled:opacity-50"
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              disabled={saving}
              className="w-full sm:w-auto inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-5 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90 transition-colors shadow-sm disabled:opacity-50"
            >
              {saving ? (
                <>
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  <span>{t('Saving...')}</span>
                </>
              ) : (
                <>
                  <Check className="mr-1.5 h-3.5 w-3.5" />
                  <span>{t('Save Configuration')}</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
