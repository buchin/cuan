import * as React from "react"
import {
  CheckCircle2,
  AlertCircle,
  Clock3,
  Loader2,
  Info,
} from "lucide-react"
import { cn } from "../../lib/utils"

export type ReportStatus = 'success' | 'failed' | 'blocked' | 'review' | 'running' | 'info'

export interface StatusConfig {
  accentBorder: string
  containerBg: string
  containerBorder: string
  badgeClasses: string
  iconColor: string
  defaultBadge: string
  defaultTitle: string
  Icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean | 'true' | 'false' }>
}

export function getReportStatusTheme(status: ReportStatus): StatusConfig {
  switch (status) {
    case 'success':
      return {
        accentBorder: 'border-l-4 border-l-emerald-500 dark:border-l-emerald-400',
        containerBg: 'bg-emerald-500/[0.06] dark:bg-emerald-500/[0.12]',
        containerBorder: 'border-emerald-500/25 dark:border-emerald-500/35',
        badgeClasses: 'bg-emerald-500/15 border-emerald-500/30 text-emerald-800 dark:text-emerald-300',
        iconColor: 'text-emerald-600 dark:text-emerald-400',
        defaultBadge: 'Success',
        defaultTitle: 'Verification Report',
        Icon: CheckCircle2,
      }
    case 'failed':
    case 'blocked':
      return {
        accentBorder: 'border-l-4 border-l-rose-500 dark:border-l-rose-400',
        containerBg: 'bg-rose-500/[0.06] dark:bg-rose-500/[0.12]',
        containerBorder: 'border-rose-500/25 dark:border-rose-500/35',
        badgeClasses: 'bg-rose-500/15 border-rose-500/30 text-rose-800 dark:text-rose-300',
        iconColor: 'text-rose-600 dark:text-rose-400',
        defaultBadge: status === 'blocked' ? 'Blocked' : 'Failed',
        defaultTitle: 'Needs Attention',
        Icon: AlertCircle,
      }
    case 'review':
      return {
        accentBorder: 'border-l-4 border-l-amber-500 dark:border-l-amber-400',
        containerBg: 'bg-amber-500/[0.06] dark:bg-amber-500/[0.12]',
        containerBorder: 'border-amber-500/25 dark:border-amber-500/35',
        badgeClasses: 'bg-amber-500/15 border-amber-500/30 text-amber-800 dark:text-amber-300',
        iconColor: 'text-amber-600 dark:text-amber-400',
        defaultBadge: 'Review',
        defaultTitle: 'Review Required',
        Icon: Clock3,
      }
    case 'running':
      return {
        accentBorder: 'border-l-4 border-l-sky-500 dark:border-l-sky-400',
        containerBg: 'bg-sky-500/[0.06] dark:bg-sky-500/[0.12]',
        containerBorder: 'border-sky-500/25 dark:border-sky-500/35',
        badgeClasses: 'bg-sky-500/15 border-sky-500/30 text-sky-800 dark:text-sky-300',
        iconColor: 'text-sky-600 dark:text-sky-400',
        defaultBadge: 'In Progress',
        defaultTitle: 'Work Update',
        Icon: Loader2,
      }
    case 'info':
    default:
      return {
        accentBorder: 'border-l-4 border-l-sky-500 dark:border-l-sky-400',
        containerBg: 'bg-muted/40 dark:bg-muted/20',
        containerBorder: 'border-border/80',
        badgeClasses: 'bg-muted border-border text-foreground',
        iconColor: 'text-muted-foreground',
        defaultBadge: 'Info',
        defaultTitle: 'Task Info',
        Icon: Info,
      }
  }
}

export interface ParsedTaskReport {
  isReport: true
  status: ReportStatus
  title: string
  badgeText: string
  body: string
}

/**
 * Parses message content to detect if it is a task verification report,
 * status checkpoint, or work update. Returns structured report data or null.
 */
export function parseTaskReportMessage(content: string, language: string = 'en'): ParsedTaskReport | null {
  if (!content || typeof content !== 'string') return null
  const trimmed = content.trim()

  // 1. Verification report prefix
  const verificationMatch = trimmed.match(
    /^(?:Based on the work session's verification report|Berdasarkan laporan verifikasi sesi kerja|Based on the verification report|Berdasarkan laporan verifikasi|Verification report|Laporan verifikasi)\s*:\s*\n*/i
  )
  if (verificationMatch) {
    const body = trimmed.slice(verificationMatch[0].length).trim()
    const isExplicitPass = /\b(?:semua kriteria penerimaan telah terpenuhi|all acceptance criteria (?:are|have been) met|all criteria met|verified complete)\b/i.test(body)
    const isFailed = !isExplicitPass && (
      /\b(?:gagal|failed|tidak terpenuhi|unmet|contradictory|blocker|terdapat error|ada error)\b/i.test(body) ||
      (/\berror\b/i.test(body) && !/\b(?:tanpa|no|zero|0|without)\s+errors?\b/i.test(body))
    )
    const isReview = !isExplicitPass && !isFailed && /\b(?:needs review|perlu ditinjau|gap:|revision required|revisi)\b/i.test(body)

    const status: ReportStatus = isFailed ? 'failed' : isReview ? 'review' : 'success'
    const title = language === 'id' ? 'Laporan Verifikasi' : 'Verification Report'
    const badgeText = isFailed
      ? (language === 'id' ? 'Gagal' : 'Failed')
      : isReview
        ? (language === 'id' ? 'Perlu Tinjauan' : 'Review')
        : (language === 'id' ? 'Berhasil' : 'Success')

    return { isReport: true, status, title, badgeText, body }
  }

  // 2. Work update checking prefix
  const workUpdateMatch = trimmed.match(
    /^(?:I[’']m checking the latest work update|Memeriksa pembaruan pekerjaan terbaru|Pembaruan pekerjaan|Work update)\s*[\.:]?\s*\n*/i
  )
  if (workUpdateMatch) {
    const body = trimmed.slice(workUpdateMatch[0].length).trim()
    const title = language === 'id' ? 'Pembaruan Pekerjaan' : 'Work Update'
    const badgeText = language === 'id' ? 'Sedang Berjalan' : 'In Progress'
    return { isReport: true, status: 'running', title, badgeText, body }
  }

  // 3. Review required prefix
  const reviewMatch = trimmed.match(
    /^(?:The result still needs review|Hasil masih perlu ditinjau|Review required|Perlu ditinjau)\s*[\.:]?\s*\n*/i
  )
  if (reviewMatch) {
    const body = trimmed.slice(reviewMatch[0].length).trim()
    const title = language === 'id' ? 'Tinjauan Hasil' : 'Review Required'
    const badgeText = language === 'id' ? 'Perlu Tinjauan' : 'Needs Review'
    return { isReport: true, status: 'review', title, badgeText, body }
  }

  // 4. Validation failed / blocked prefix
  const failedMatch = trimmed.match(
    /^(?:The result review could not be validated|Laporan verifikasi tidak dapat divalidasi|Validation failed|Tugas tertahan)\s*[\.:]?\s*\n*/i
  )
  if (failedMatch) {
    const body = trimmed.slice(failedMatch[0].length).trim()
    const title = language === 'id' ? 'Verifikasi Tertahan' : 'Validation Blocked'
    const badgeText = language === 'id' ? 'Tertahan' : 'Blocked'
    return { isReport: true, status: 'blocked', title, badgeText, body }
  }

  // 5. Markdown task report header e.g. "### Task Report: Success" or "### Status Report"
  const headerMatch = trimmed.match(
    /^(?:###|##)\s*(?:Task Report|Status Report|Laporan Tugas|Laporan Status)(?:\s*[:\-]\s*([^\n]+))?\n([\s\S]*)$/i
  )
  if (headerMatch) {
    const explicitStatus = (headerMatch[1] || '').trim().toLowerCase()
    let status: ReportStatus = 'info'
    if (/success|selesai|done|passed|berhasil/i.test(explicitStatus)) status = 'success'
    else if (/failed|gagal|blocked|error|tertahan/i.test(explicitStatus)) status = 'blocked'
    else if (/review|tinjau/i.test(explicitStatus)) status = 'review'
    else if (/running|doing|progress|berjalan/i.test(explicitStatus)) status = 'running'

    const body = headerMatch[2].trim()
    const title = headerMatch[1]?.trim() || (language === 'id' ? 'Laporan Tugas' : 'Task Report')
    const badgeText = status === 'success'
      ? (language === 'id' ? 'Berhasil' : 'Success')
      : status === 'blocked'
        ? (language === 'id' ? 'Tertahan' : 'Blocked')
        : status === 'review'
          ? (language === 'id' ? 'Perlu Tinjauan' : 'Review')
          : (language === 'id' ? 'Sedang Berjalan' : 'In Progress')

    return { isReport: true, status, title, badgeText, body }
  }

  return null
}

export interface StatusReportCardProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'title'> {
  status: ReportStatus
  title?: React.ReactNode
  badgeText?: string
  headerIcon?: React.ReactNode
  actions?: React.ReactNode
  timestamp?: string | React.ReactNode
  children?: React.ReactNode
  compact?: boolean
}

export const StatusReportCard = React.forwardRef<HTMLDivElement, StatusReportCardProps>(
  (
    {
      status,
      title,
      badgeText,
      headerIcon,
      actions,
      timestamp,
      children,
      compact = false,
      className,
      ...props
    },
    ref
  ) => {
    const theme = getReportStatusTheme(status)
    const Icon = theme.Icon

    return (
      <div
        ref={ref}
        data-report-status={status}
        className={cn(
          "status-report-card relative w-full min-w-0 max-w-full rounded-lg border text-sm transition-colors overflow-hidden break-words shadow-xs",
          theme.accentBorder,
          theme.containerBg,
          theme.containerBorder,
          compact ? "p-2.5 sm:p-3" : "p-3 sm:p-4",
          className
        )}
        {...props}
      >
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-1.5 sm:gap-2 border-b border-border/40 pb-2 mb-2.5">
          <div className="flex items-center gap-1.5 sm:gap-2 min-w-0 max-w-full">
            {headerIcon || (
              <Icon
                className={cn(
                  "h-4 w-4 shrink-0",
                  theme.iconColor,
                  status === "running" && "animate-spin"
                )}
                aria-hidden="true"
              />
            )}
            <span className="font-semibold text-xs sm:text-sm text-foreground truncate">
              {title || theme.defaultTitle}
            </span>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <span
              className={cn(
                "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] sm:text-[11px] font-semibold tracking-wide uppercase",
                theme.badgeClasses
              )}
            >
              {badgeText || theme.defaultBadge}
            </span>
            {timestamp && (
              <span className="text-[11px] text-muted-foreground tabular-nums">
                {timestamp}
              </span>
            )}
          </div>
        </div>

        {/* Content Body */}
        <div className="min-w-0 max-w-full text-foreground/90 leading-relaxed break-words">
          {children}
        </div>

        {/* Optional Actions */}
        {actions && (
          <div className="mt-3 pt-2.5 border-t border-border/40 flex flex-wrap items-center gap-2">
            {actions}
          </div>
        )}
      </div>
    )
  }
)
StatusReportCard.displayName = "StatusReportCard"
