import { useState, useEffect, useRef } from 'react'
import { Radio, Users, Mail, X, Check, AlertCircle } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import { BackAction } from '../../components/layout/DetailLayout'
import { IconButton } from '../../components/ui/IconButton'

export type BroadcastRecipient = {
  email: string
  name?: string
  source: 'crm' | 'custom' | 'both'
  contact_id?: string
  tags?: string[]
}

export type BroadcastAudienceResponse = {
  total_unique: number
  crm_count: number
  custom_count: number
  recipients: BroadcastRecipient[]
}

export type BroadcastTagSummary = {
  tag: string
  count: number
}

interface BroadcastAudiencePanelProps {
  onClose?: () => void
  initialTags?: string[]
}

export function BroadcastAudiencePanel({ onClose, initialTags = [] }: BroadcastAudiencePanelProps) {
  const { t } = useLanguage()
  const [availableTags, setAvailableTags] = useState<BroadcastTagSummary[]>([])
  const [tagsLoading, setTagsLoading] = useState(true)
  const [selectedTags, setSelectedTags] = useState<string[]>(initialTags)
  const [customEmails, setCustomEmails] = useState('')
  const [audience, setAudience] = useState<BroadcastAudienceResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const abortControllerRef = useRef<AbortController | null>(null)

  // Load CRM tags summary
  useEffect(() => {
    let active = true
    setTagsLoading(true)
    apiFetch<{ tags: BroadcastTagSummary[] }>('/api/v1/contacts/broadcast/tags')
      .then((data) => {
        if (active) {
          setAvailableTags(data.tags || [])
        }
      })
      .catch(() => {
        // Fallback: silently retain empty tags
      })
      .finally(() => {
        if (active) setTagsLoading(false)
      })
    return () => {
      active = false
    }
  }, [])

  // Debounced Audience Resolution
  useEffect(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }

    const controller = new AbortController()
    abortControllerRef.current = controller

    const timer = setTimeout(() => {
      // If neither tags nor custom emails, resolve with empty payload to get 0 count
      setLoading(true)
      setError('')

      apiFetch<BroadcastAudienceResponse>('/api/v1/contacts/broadcast/audience', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tags: selectedTags,
          custom_emails: customEmails,
        }),
        signal: controller.signal,
      })
        .then((res) => {
          if (!controller.signal.aborted) {
            setAudience(res)
            setError('')
          }
        })
        .catch((err) => {
          if (!controller.signal.aborted) {
            setError(err instanceof Error ? err.message : t('Gagal memuat pratinjau penerima'))
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) {
            setLoading(false)
          }
        })
    }, 250)

    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [selectedTags, customEmails, t])

  const toggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag],
    )
  }

  const clearAll = () => {
    setSelectedTags([])
    setCustomEmails('')
  }

  return (
    <aside
      tabIndex={-1}
      aria-label={t('Broadcast Email')}
      className="mx-auto min-w-0 w-full max-w-[75ch] outline-none space-y-6"
    >
      {onClose && (
        <BackAction className="min-[1200px]:hidden" onClick={onClose}>
          {t('Back to contacts')}
        </BackAction>
      )}

      {/* Header: Serene Bear Note Pad Title */}
      <header className="border-b border-border/40 pb-5">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Radio className="h-4 w-4" />
              </span>
              <h2 className="text-lg sm:text-xl font-semibold tracking-tight text-foreground font-sans">
                {t('Broadcast Email')}
              </h2>
            </div>
            <p className="text-xs text-muted-foreground">
              {t('Kirim pengumuman ke segmen tag kontak CRM dan alamat email kustom')}
            </p>
          </div>

          <div className="flex items-center gap-2">
            {(selectedTags.length > 0 || customEmails.trim()) && (
              <button
                type="button"
                onClick={clearAll}
                className="action-secondary text-xs px-2.5 py-1"
              >
                {t('Reset')}
              </button>
            )}
            {onClose && (
              <IconButton
                type="button"
                label={t('Close broadcast')}
                title={t('Close broadcast')}
                onClick={onClose}
              >
                <X className="h-4 w-4" />
              </IconButton>
            )}
          </div>
        </div>
      </header>

      {/* Main Form & Preview Canvas */}
      <div className="space-y-6">
        {/* Section 1: Tag Segment Selector */}
        <section aria-labelledby="tag-segment-heading" className="space-y-3">
          <div className="flex items-center justify-between">
            <h3
              id="tag-segment-heading"
              className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5"
            >
              <Users className="h-3.5 w-3.5" />
              <span>{t('Pilih Segmen Tag CRM')}</span>
            </h3>
            {selectedTags.length > 0 && (
              <span className="text-[11px] text-muted-foreground">
                {selectedTags.length} {t('tag terpilih')}
              </span>
            )}
          </div>

          {tagsLoading ? (
            <p className="text-xs text-muted-foreground py-2">{t('Memuat tag kontak…')}</p>
          ) : availableTags.length === 0 ? (
            <p className="text-xs text-muted-foreground italic py-1">
              {t('Belum ada tag pada kontak CRM. Tambahkan tag pada kontak terlebih dahulu.')}
            </p>
          ) : (
            <div className="flex flex-wrap gap-2 pt-1" role="group" aria-label={t('Pilihan tag kontak')}>
              {availableTags.map(({ tag, count }) => {
                const isSelected = selectedTags.includes(tag)
                return (
                  <button
                    key={tag}
                    type="button"
                    role="checkbox"
                    aria-checked={isSelected}
                    onClick={() => toggleTag(tag)}
                    className={`inline-flex min-h-[36px] items-center gap-1.5 rounded-full px-3 py-1 text-xs transition-colors border ${
                      isSelected
                        ? 'bg-primary text-primary-foreground border-primary font-medium'
                        : 'bg-muted/40 text-foreground border-border/40 hover:bg-muted/70'
                    }`}
                  >
                    <span className={isSelected ? 'text-primary-foreground/80' : 'text-muted-foreground'}>
                      #
                    </span>
                    <span>{tag}</span>
                    <span
                      className={`ml-0.5 rounded-full px-1.5 py-0.2 text-[10px] tabular-nums ${
                        isSelected
                          ? 'bg-primary-foreground/20 text-primary-foreground'
                          : 'bg-muted text-muted-foreground'
                      }`}
                    >
                      {count}
                    </span>
                    {isSelected && <Check className="h-3 w-3 ml-0.5" />}
                  </button>
                )
              })}
            </div>
          )}
        </section>

        {/* Section 2: Custom Emails Input */}
        <section aria-labelledby="custom-emails-heading" className="space-y-2 border-t border-border/40 pt-5">
          <div className="flex items-center justify-between">
            <h3
              id="custom-emails-heading"
              className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5"
            >
              <Mail className="h-3.5 w-3.5" />
              <span>{t('Alamat Email Kustom Tambahan')}</span>
            </h3>
            <span className="text-[11px] text-muted-foreground">
              {t('Opsional')}
            </span>
          </div>

          <p className="text-xs text-muted-foreground">
            {t('Masukkan alamat email tambahan (pisahkan dengan koma, spasi, atau baris baru).')}
          </p>

          <textarea
            id="custom-emails-textarea"
            aria-label={t('Alamat email kustom')}
            rows={3}
            value={customEmails}
            onChange={(e) => setCustomEmails(e.target.value)}
            placeholder="misal: investor@example.com, partner@example.com"
            className="field-control w-full font-mono text-xs leading-relaxed resize-y min-h-[72px]"
          />
        </section>

        {/* Error notification */}
        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive"
          >
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <p className="flex-1">{t(error)}</p>
          </div>
        )}

        {/* Section 3: Recipient Audience Preview */}
        <section aria-labelledby="audience-preview-heading" className="border-t border-border/40 pt-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <h3
                id="audience-preview-heading"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
              >
                {t('Pratinjau Penerima')}
              </h3>
              {loading && (
                <span className="text-xs text-muted-foreground">({t('Memperbarui…')})</span>
              )}
            </div>

            {audience && (
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <span className="inline-flex items-center rounded-full bg-muted/60 px-2.5 py-0.5 font-medium text-foreground border border-border/40">
                  <span className="font-semibold text-primary mr-1 tabular-nums">
                    {audience.total_unique}
                  </span>
                  {t('penerima unik')}
                </span>
                <span className="text-[11px] text-muted-foreground">
                  ({audience.crm_count} CRM · {audience.custom_count} kustom)
                </span>
              </div>
            )}
          </div>

          {/* Recipient List or Empty State */}
          {!selectedTags.length && !customEmails.trim() ? (
            <div className="rounded-lg border border-dashed border-border/60 p-6 text-center bg-muted/5">
              <p className="text-xs text-muted-foreground">
                {t('Pilih tag kontak di atas atau ketik email kustom untuk melihat pratinjau daftar penerima.')}
              </p>
            </div>
          ) : audience && audience.total_unique === 0 && !loading ? (
            <div className="rounded-lg border border-border/40 p-4 text-center bg-muted/5">
              <p className="text-xs text-muted-foreground">
                {t('Tidak ada penerima yang cocok dengan kriteria yang dipilih.')}
              </p>
            </div>
          ) : audience && audience.recipients.length > 0 ? (
            <div className="space-y-1">
              <div className="flex items-center justify-between px-1 text-[11px] text-muted-foreground">
                <span>{t('Daftar Penerima Unik')}</span>
                <span className="text-emerald-600 dark:text-emerald-400 font-medium">
                  {t('Deduplikasi otomatis')} ✓
                </span>
              </div>

              <div className="divide-y divide-border/30 rounded-lg border border-border/40 overflow-hidden bg-card/30">
                {audience.recipients.map((rec) => {
                  const initial = (rec.name || rec.email || '?')
                    .slice(0, 2)
                    .toUpperCase()
                  return (
                    <div
                      key={rec.email}
                      className="flex items-center justify-between gap-3 px-3 py-2 text-left hover:bg-muted/20 transition-colors"
                    >
                      <div className="flex items-center gap-2.5 min-w-0">
                        <span
                          aria-hidden="true"
                          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-semibold text-foreground/80"
                        >
                          {initial}
                        </span>
                        <div className="min-w-0 flex-1">
                          {rec.name && rec.name !== rec.email ? (
                            <>
                              <p className="truncate text-xs font-medium text-foreground">
                                {rec.name}
                              </p>
                              <p className="truncate font-mono text-[11px] text-muted-foreground">
                                {rec.email}
                              </p>
                            </>
                          ) : (
                            <p className="truncate font-mono text-xs text-foreground">
                              {rec.email}
                            </p>
                          )}
                        </div>
                      </div>

                      <div className="flex items-center gap-1.5 shrink-0">
                        {rec.tags && rec.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {rec.tags.map((tg) => (
                              <span
                                key={tg}
                                className="inline-flex items-center rounded-full bg-muted/60 px-2 py-0.5 text-[10px] text-muted-foreground border border-border/40 font-mono"
                              >
                                #{tg.replace(/^#/, '')}
                              </span>
                            ))}
                          </div>
                        )}
                        {rec.source === 'custom' && (
                          <span className="inline-flex items-center rounded-full bg-blue-500/10 text-blue-600 dark:text-blue-400 px-2 py-0.5 text-[10px] border border-blue-500/20 font-medium">
                            {t('Kustom')}
                          </span>
                        )}
                        {rec.source === 'both' && (
                          <span className="inline-flex items-center rounded-full bg-purple-500/10 text-purple-600 dark:text-purple-400 px-2 py-0.5 text-[10px] border border-purple-500/20 font-medium">
                            {t('CRM + Kustom')}
                          </span>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          ) : null}
        </section>
      </div>
    </aside>
  )
}
