import { useState } from 'react'
import {
  CheckCircle2,
  AlertCircle,
  Loader2,
  Terminal,
  FileEdit,
  FileText,
  Search,
  Brain,
  Clock,
  Bot,
  Wrench,
  ChevronDown,
  ChevronUp,
  Copy,
  Check,
} from 'lucide-react'
import type { VisualStep, StepIconType, StepStatus } from './ndjsonFormatter'

function getStepIcon(icon: StepIconType, status: StepStatus) {
  if (status === 'running') {
    return <Loader2 className="h-4 w-4 animate-spin text-primary" aria-label="Running" />
  }
  if (status === 'failed') {
    return <AlertCircle className="h-4 w-4 text-rose-400" aria-label="Failed" />
  }
  if (status === 'completed') {
    return <CheckCircle2 className="h-4 w-4 text-emerald-500" aria-label="Completed" />
  }

  switch (icon) {
    case 'command':
      return <Terminal className="h-4 w-4 text-sky-400" />
    case 'file-write':
      return <FileEdit className="h-4 w-4 text-amber-400" />
    case 'file-read':
      return <FileText className="h-4 w-4 text-blue-400" />
    case 'search':
      return <Search className="h-4 w-4 text-violet-400" />
    case 'thought':
      return <Brain className="h-4 w-4 text-pink-400" />
    case 'clock':
      return <Clock className="h-4 w-4 text-amber-400" />
    case 'subagent':
      return <Bot className="h-4 w-4 text-indigo-400" />
    default:
      return <Wrench className="h-4 w-4 text-muted-foreground" />
  }
}

export function VisualStepsList({
  steps,
  onCopyText,
  className = '',
}: {
  steps: VisualStep[]
  onCopyText?: (text: string) => void
  className?: string
}) {
  const [expandedIds, setExpandedIds] = useState<Record<string, boolean>>({})
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const toggleExpand = (id: string) => {
    setExpandedIds((prev) => ({
      ...prev,
      [id]: !prev[id],
    }))
  }

  const handleCopy = (id: string, text: string) => {
    if (onCopyText) {
      onCopyText(text)
    } else {
      void navigator.clipboard?.writeText(text)
    }
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  if (steps.length === 0) {
    return (
      <div className={`flex flex-col items-center justify-center py-12 px-4 text-center select-none ${className}`}>
        <div className="h-10 w-10 rounded-full bg-muted/20 border border-border/40 flex items-center justify-center text-muted-foreground mb-3">
          <Terminal className="h-5 w-5 opacity-60" />
        </div>
        <p className="text-sm font-medium text-foreground/80">Belum ada langkah visual</p>
        <p className="text-xs text-muted-foreground mt-1 max-w-sm">
          Log proses agen akan diparsing otomatis menjadi langkah kerja yang rapi dan mudah dibaca di sini.
        </p>
      </div>
    )
  }

  return (
    <div
      role="list"
      aria-label="Langkah eksekusi visual"
      className={`space-y-2 p-2 sm:p-3 overflow-y-auto overscroll-contain select-text ${className}`}
    >
      {steps.map((step, idx) => {
        const isExpanded = Boolean(expandedIds[step.id])
        return (
          <div
            key={step.id || idx}
            role="listitem"
            className="rounded-xl border border-border/40 bg-card/40 hover:bg-card/70 transition-colors shadow-2xs overflow-hidden"
          >
            {/* Header Row */}
            <div
              onClick={() => toggleExpand(step.id)}
              className="flex items-center gap-2.5 px-3 py-2.5 cursor-pointer select-none"
            >
              {/* Step Icon */}
              <div className="shrink-0 flex items-center justify-center">
                {getStepIcon(step.icon, step.status)}
              </div>

              {/* Step Number & Tool Info */}
              <div className="min-w-0 flex-1 flex flex-col sm:flex-row sm:items-center gap-0.5 sm:gap-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <span className="text-[11px] font-mono font-semibold text-muted-foreground/70 shrink-0">
                    #{step.stepNumber ?? idx + 1}
                  </span>
                  <span className="text-xs sm:text-sm font-medium text-foreground truncate">
                    {step.toolDisplayName}
                  </span>
                </div>
                <span className="text-xs text-muted-foreground truncate font-mono">
                  {step.summary}
                </span>
              </div>

              {/* Duration & Status Badge */}
              <div className="shrink-0 flex items-center gap-1.5">
                {step.duration && (
                  <span className="inline-flex items-center rounded-md bg-muted/40 px-1.5 py-0.5 text-[11px] font-mono text-muted-foreground">
                    {step.duration}
                  </span>
                )}
                <button
                  type="button"
                  aria-label={isExpanded ? 'Tutup rincian' : 'Buka rincian'}
                  className="p-1 text-muted-foreground hover:text-foreground rounded transition-colors"
                >
                  {isExpanded ? (
                    <ChevronUp className="h-3.5 w-3.5" />
                  ) : (
                    <ChevronDown className="h-3.5 w-3.5" />
                  )}
                </button>
              </div>
            </div>

            {/* Collapsible Details Drawer */}
            {isExpanded && (
              <div className="border-t border-border/30 bg-black/20 p-3 space-y-2 text-xs">
                {step.details && (
                  <div className="space-y-1">
                    <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                      <span className="font-mono uppercase tracking-wider">Parameter / Keluaran</span>
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleCopy(step.id, step.details || '')
                        }}
                        className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {copiedId === step.id ? (
                          <>
                            <Check className="h-3 w-3 text-emerald-400" />
                            <span>Tersalin</span>
                          </>
                        ) : (
                          <>
                            <Copy className="h-3 w-3" />
                            <span>Salin</span>
                          </>
                        )}
                      </button>
                    </div>
                    <pre className="max-h-48 overflow-x-auto overflow-y-auto rounded-lg bg-neutral-950 p-2 font-mono text-[11px] text-neutral-300 border border-border/20 whitespace-pre-wrap break-all">
                      {step.details}
                    </pre>
                  </div>
                )}
                {step.raw && !step.details && (
                  <pre className="max-h-48 overflow-x-auto overflow-y-auto rounded-lg bg-neutral-950 p-2 font-mono text-[11px] text-neutral-400 border border-border/20 whitespace-pre-wrap break-all">
                    {step.raw}
                  </pre>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
