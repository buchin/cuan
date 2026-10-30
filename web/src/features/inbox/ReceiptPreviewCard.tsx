import { useState } from 'react'
import {
  Receipt,
  Store,
  Calendar,
  Tag,
  CheckCircle2,
  CheckCheck,
  Sparkles,
  ShoppingBag,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import type { ReceiptExtraction, ReceiptItem } from '../../api/appsSkills'
import { cn } from '../../lib/utils'

export interface ReceiptPreviewCardProps {
  extraction: ReceiptExtraction
  onConfirm?: (extraction: ReceiptExtraction) => void | Promise<void>
  disabled?: boolean
  className?: string
}

export function formatCurrencyIDR(amount: number, currency = 'IDR'): string {
  try {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: currency || 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount)
  } catch {
    return `Rp ${amount.toLocaleString('id-ID')}`
  }
}

/**
 * Extracts structured receipt extraction data from message content if formatted
 * as ```cuan-receipt, ```receipt, or a recognized JSON structure.
 */
export function parseReceiptExtraction(content: string): ReceiptExtraction | null {
  if (!content) return null

  // 1. Check for ```cuan-receipt or ```receipt code block
  const fenceMatch = content.match(/```(?:cuan-receipt|receipt)\s*([\s\S]*?)```/)
  if (fenceMatch && fenceMatch[1]) {
    try {
      const parsed = JSON.parse(fenceMatch[1].trim())
      if (parsed && (parsed.merchant || parsed.total_amount !== undefined)) {
        return sanitizeExtraction(parsed)
      }
    } catch {
      // not valid JSON
    }
  }

  // 2. Check for generic ```json block with receipt fields
  const jsonMatch = content.match(/```json\s*([\s\S]*?)```/)
  if (jsonMatch && jsonMatch[1]) {
    try {
      const parsed = JSON.parse(jsonMatch[1].trim())
      if (
        parsed &&
        typeof parsed === 'object' &&
        (parsed.merchant || parsed.items || parsed.total_amount !== undefined) &&
        (parsed.currency || parsed.total_amount || parsed.category)
      ) {
        return sanitizeExtraction(parsed)
      }
    } catch {
      // not valid JSON
    }
  }

  // 3. Fallback: Check if the entire content is raw JSON with receipt data
  const trimmed = content.trim()
  if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
    try {
      const parsed = JSON.parse(trimmed)
      if (parsed && parsed.merchant && (parsed.total_amount !== undefined || parsed.items)) {
        return sanitizeExtraction(parsed)
      }
    } catch {
      // not JSON
    }
  }

  return null
}

function sanitizeExtraction(raw: any): ReceiptExtraction {
  const items: ReceiptItem[] = Array.isArray(raw.items)
    ? raw.items.map((it: any) => ({
        name: String(it.name || 'Item'),
        quantity: Number(it.quantity) || 1,
        price: Number(it.price) || 0,
        total: Number(it.total) || (Number(it.quantity) || 1) * (Number(it.price) || 0),
      }))
    : []

  const totalAmount =
    typeof raw.total_amount === 'number'
      ? raw.total_amount
      : typeof raw.total === 'number'
        ? raw.total
        : items.reduce((acc, it) => acc + it.total, 0)

  return {
    merchant: String(raw.merchant || 'Toko / Merchant'),
    date: String(raw.date || new Date().toISOString().split('T')[0]),
    total_amount: totalAmount,
    currency: String(raw.currency || 'IDR'),
    items,
    category: String(raw.category || 'Belanja Operasional'),
    confidence: typeof raw.confidence === 'number' ? raw.confidence : 0.85,
    raw_text: raw.raw_text,
    confirmed: Boolean(raw.confirmed),
    recorded_at: raw.recorded_at,
  }
}

export function ReceiptPreviewCard({
  extraction,
  onConfirm,
  disabled = false,
  className,
}: ReceiptPreviewCardProps) {
  const { t } = useLanguage()
  const [confirmed, setConfirmed] = useState(Boolean(extraction.confirmed))
  const [loading, setLoading] = useState(false)

  const handleConfirm = async () => {
    if (confirmed || loading || disabled) return
    setLoading(true)
    try {
      if (onConfirm) {
        await onConfirm({
          ...extraction,
          confirmed: true,
          recorded_at: new Date().toISOString(),
        })
      }
      setConfirmed(true)
    } finally {
      setLoading(false)
    }
  }

  const confidencePct = Math.round((extraction.confidence || 0.85) * 100)

  return (
    <div
      className={cn(
        'my-3 w-full max-w-full rounded-xl border border-border/80 bg-card/95 text-foreground shadow-xs overflow-hidden transition-all',
        confirmed
          ? 'border-emerald-500/40 bg-emerald-500/[0.03] dark:bg-emerald-950/10'
          : 'border-border/80 hover:border-primary/40',
        className
      )}
      data-testid="receipt-preview-card"
    >
      {/* Header Bar */}
      <div className="flex items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3.5 py-2.5">
        <div className="flex items-center gap-2 min-w-0">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Receipt className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="text-xs font-bold text-foreground">
                {t('Receipt Extraction')}
              </span>
              <span className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-semibold bg-primary/10 text-primary border border-primary/20">
                <Sparkles className="h-2.5 w-2.5" />
                <span>Vision</span>
              </span>
            </div>
            <p className="text-[10px] text-muted-foreground truncate">
              {t('AI-powered receipt & invoice analyzer')}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1.5 shrink-0">
          {confidencePct > 0 && (
            <span
              className={cn(
                'text-[10px] font-mono px-2 py-0.5 rounded border',
                confidencePct >= 80
                  ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                  : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
              )}
              title={t('Recognition confidence level')}
            >
              {confidencePct}% {t('Confidence')}
            </span>
          )}
        </div>
      </div>

      {/* Main Content Details */}
      <div className="p-3.5 space-y-3">
        {/* Merchant, Date & Category */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
          {/* Merchant */}
          <div className="flex items-start gap-2 rounded-lg border border-border/50 bg-background/60 p-2 min-w-0">
            <Store className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <div className="min-w-0">
              <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground block">
                {t('Vendor / Merchant')}
              </span>
              <span className="text-xs font-bold text-foreground truncate block" title={extraction.merchant}>
                {extraction.merchant}
              </span>
            </div>
          </div>

          {/* Date */}
          <div className="flex items-start gap-2 rounded-lg border border-border/50 bg-background/60 p-2 min-w-0">
            <Calendar className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <div className="min-w-0">
              <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground block">
                {t('Date')}
              </span>
              <span className="text-xs font-medium text-foreground truncate block font-mono">
                {extraction.date}
              </span>
            </div>
          </div>

          {/* Category */}
          <div className="flex items-start gap-2 rounded-lg border border-border/50 bg-background/60 p-2 min-w-0">
            <Tag className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <div className="min-w-0">
              <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground block">
                {t('Category')}
              </span>
              <span className="text-xs font-medium text-foreground truncate block">
                {extraction.category}
              </span>
            </div>
          </div>
        </div>

        {/* Itemized Line Items List */}
        {extraction.items && extraction.items.length > 0 && (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <ShoppingBag className="h-3.5 w-3.5 text-primary" />
                <span>{t('Purchased Items')} ({extraction.items.length})</span>
              </span>
            </div>

            <div className="rounded-lg border border-border/60 bg-muted/20 overflow-hidden divide-y divide-border/40 text-xs">
              {extraction.items.map((item, idx) => (
                <div key={idx} className="flex items-center justify-between p-2 hover:bg-muted/30 transition-colors">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="text-[11px] font-mono text-muted-foreground shrink-0 px-1 rounded bg-muted/60">
                      {item.quantity}x
                    </span>
                    <span className="text-foreground font-medium truncate" title={item.name}>
                      {item.name}
                    </span>
                  </div>
                  <div className="text-right shrink-0 font-mono text-xs font-semibold text-foreground pl-2">
                    {formatCurrencyIDR(item.total, extraction.currency)}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Total Amount Box */}
        <div className="flex items-center justify-between p-3 rounded-lg bg-primary/[0.04] border border-primary/20">
          <div>
            <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground block">
              {t('Total Amount')}
            </span>
            <span className="text-[10px] text-muted-foreground">
              {t('Includes tax & service if applicable')}
            </span>
          </div>
          <div className="text-right">
            <span className="text-base sm:text-lg font-bold font-mono text-primary block" data-testid="receipt-total-amount">
              {formatCurrencyIDR(extraction.total_amount, extraction.currency)}
            </span>
          </div>
        </div>

        {/* Human-in-the-Loop Action Confirmation */}
        <div className="pt-1">
          {confirmed ? (
            <div className="w-full min-h-11 py-2 px-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400 text-xs font-bold flex items-center justify-center gap-2 animate-in fade-in">
              <CheckCheck className="h-4 w-4 shrink-0" />
              <span>{t('Confirmed & Recorded in Ledger')}</span>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleConfirm}
              disabled={disabled || loading}
              className="w-full min-h-11 inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring active:scale-[0.99] transition-all shadow-xs disabled:opacity-50 cursor-pointer"
              aria-label={t('Confirm & Record')}
              data-testid="confirm-record-button"
            >
              {loading ? (
                <span>{t('Recording...')}</span>
              ) : (
                <>
                  <CheckCircle2 className="h-4 w-4" />
                  <span>{t('Confirm & Record')}</span>
                </>
              )}
            </button>
          )}
          <p className="mt-1.5 text-center text-[10px] text-muted-foreground">
            {t('Human-in-the-Loop: Review details before recording to your Cuan ledger and Google Sheets.')}
          </p>
        </div>
      </div>
    </div>
  )
}
