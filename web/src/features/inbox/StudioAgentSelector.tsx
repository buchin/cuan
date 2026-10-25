import { useState, useRef, useEffect } from 'react'
import { Bot, ChevronDown, Check, Zap, Sparkles, Eye } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { type Agent, isModelVisionCapable } from '../../api/agents'
import { connectionStatus } from '../agents/connection'

export function StudioAgentSelector({
  agents,
  selectedAgentId,
  onSelect,
  disabled,
}: {
  agents: Agent[]
  selectedAgentId: string
  onSelect: (id: string) => void
  disabled?: boolean
}) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Close when clicking outside or pressing Escape
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen])

  const selectedAgent = agents.find((a) => a.id === selectedAgentId)

  // Display label on the pill button
  const pillLabel = selectedAgent ? selectedAgent.name : t('Auto (choose best)')
  const pillIcon = selectedAgent ? (
    <Bot className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
  ) : (
    <Sparkles className="h-3.5 w-3.5 text-amber-500" aria-hidden="true" />
  )

  return (
    <div className="relative inline-block text-left" ref={containerRef}>
      <button
        type="button"
        id="studio-agent-selector-pill"
        onClick={() => !disabled && setIsOpen((prev) => !prev)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-label={t('Select Agent')}
        className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-muted/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
      >
        {pillIcon}
        <span className="truncate max-w-[120px] sm:max-w-[160px]">{pillLabel}</span>
        <ChevronDown className={`h-3 w-3 text-muted-foreground transition-transform duration-150 ${isOpen ? 'rotate-180' : ''}`} aria-hidden="true" />
      </button>

      {isOpen && (
        <div
          role="listbox"
          aria-label={t('Available Agents')}
          className="absolute bottom-full left-0 mb-2 z-50 w-72 sm:w-80 rounded-xl border border-border bg-popover p-1.5 shadow-xl outline-none max-h-72 overflow-y-auto"
        >
          {/* Option: Auto */}
          <button
            type="button"
            role="option"
            aria-selected={!selectedAgentId}
            onClick={() => {
              onSelect('')
              setIsOpen(false)
            }}
            className={`w-full flex items-start gap-2.5 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
              !selectedAgentId ? 'bg-accent/60 font-semibold' : ''
            }`}
          >
            <div className="mt-0.5 rounded-md bg-amber-500/10 p-1 text-amber-500 shrink-0">
              <Sparkles className="h-3.5 w-3.5" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <span className="font-medium text-foreground">{t('Auto (choose best)')}</span>
                <span className="text-[10px] uppercase font-bold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                  {t('Recommended')}
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground mt-0.5 leading-tight">
                {t('Cuan automatically selects the optimal agent and manages quota failover.')}
              </p>
            </div>
            {!selectedAgentId && <Check className="h-4 w-4 text-primary shrink-0 self-center" />}
          </button>

          <div className="my-1 border-t border-border/50" />

          {/* Individual Agents */}
          {agents.map((agent) => {
            const state = connectionStatus(agent, Date.now())
            const vision = isModelVisionCapable(agent.model_id, agent.provider || agent.harness)
            const isSelected = agent.id === selectedAgentId
            const isInactive = agent.enabled === false || agent.status === 'inactive' || state.state === 'disabled' || state.state === 'login_required'

            const modelLower = (agent.model_id || '').toLowerCase()
            const isThinking = modelLower.includes('pro') || modelLower.includes('opus') || modelLower.includes('thinking') || modelLower.includes('sonnet')

            return (
              <button
                key={agent.id}
                type="button"
                role="option"
                disabled={isInactive}
                aria-selected={isSelected}
                onClick={() => {
                  onSelect(agent.id)
                  setIsOpen(false)
                }}
                className={`w-full flex items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground disabled:opacity-40 disabled:pointer-events-none ${
                  isSelected ? 'bg-accent/60 font-semibold' : ''
                }`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  {/* Connection Status dot */}
                  <span
                    className={`h-2 w-2 shrink-0 rounded-full ${
                      state.state === 'verified'
                        ? 'bg-emerald-500'
                        : state.state === 'quota_limited'
                        ? 'bg-amber-500'
                        : 'bg-zinc-400'
                    }`}
                    title={t(state.label)}
                  />
                  <div className="truncate">
                    <span className="font-medium text-foreground">{agent.name}</span>
                    <span className="ml-1.5 text-[10px] text-muted-foreground">
                      {t(state.label)}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-1 shrink-0">
                  {isThinking ? (
                    <span className="inline-flex items-center gap-0.5 rounded bg-purple-500/10 px-1.5 py-0.5 text-[10px] font-medium text-purple-600 dark:text-purple-400">
                      <Zap className="h-2.5 w-2.5" />
                      Pro
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-0.5 rounded bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-600 dark:text-blue-400">
                      <Zap className="h-2.5 w-2.5" />
                      Fast
                    </span>
                  )}
                  {vision && (
                    <span className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground" title={t('Vision capable')}>
                      <Eye className="h-2.5 w-2.5 mr-0.5" />
                      Vision
                    </span>
                  )}
                  {isSelected && <Check className="h-4 w-4 text-primary ml-1" />}
                </div>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
