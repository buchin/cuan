import { useState, useEffect, useRef } from 'react'
import {
  Radio,
  Users,
  Mail,
  X,
  Check,
  AlertCircle,
  Send,
  Loader2,
  ShieldCheck,
  PenLine,
  History,
  Clock,
  ChevronRight,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import { BackAction } from '../../components/layout/DetailLayout'
import { IconButton } from '../../components/ui/IconButton'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { FormField, Input, Textarea } from '../../components/ui/forms'

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

export type BroadcastSendResponse = {
  broadcast_id?: string
  status: 'success' | 'partial' | 'failed'
  total_recipients: number
  sent_count: number
  failed_count: number
  errors?: string[]
  idempotency_key: string
  already_sent?: boolean
  message: string
}

export type CRMBroadcastLog = {
  id: string
  broadcast_id: string
  email: string
  name: string
  source: 'crm' | 'custom' | 'both'
  status: 'sent' | 'failed'
  error_message?: string
  sent_at: string
}

export type CRMBroadcastSession = {
  id: string
  idempotency_key: string
  subject: string
  content: string
  tags: string[]
  custom_emails?: string[]
  total_recipients: number
  sent_count: number
  failed_count: number
  status: 'success' | 'partial' | 'failed'
  message: string
  created_at: string
  updated_at: string
  logs?: CRMBroadcastLog[]
}

const formatTimestamp = (iso: string, language: string) => {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString(language === 'id' ? 'id-ID' : 'en-US', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

interface BroadcastAudiencePanelProps {
  onClose?: () => void
  initialTags?: string[]
  initialTab?: 'compose' | 'history'
}

export function BroadcastAudiencePanel({ onClose, initialTags = [], initialTab = 'compose' }: BroadcastAudiencePanelProps) {
  const { t, language } = useLanguage()
  const [activeTab, setActiveTab] = useState<'compose' | 'history'>(initialTab)
  const [availableTags, setAvailableTags] = useState<BroadcastTagSummary[]>([])
  const [tagsLoading, setTagsLoading] = useState(true)
  const [selectedTags, setSelectedTags] = useState<string[]>(initialTags)
  const [customEmails, setCustomEmails] = useState('')
  const [audience, setAudience] = useState<BroadcastAudienceResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const abortControllerRef = useRef<AbortController | null>(null)

  // Message composer & sending state
  const [subject, setSubject] = useState('')
  const [content, setContent] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState('')
  const [sendSuccess, setSendSuccess] = useState<BroadcastSendResponse | null>(null)
  const idempotencyKeyRef = useRef<string>(crypto.randomUUID())

  // History state (Phase 3)
  const [historyList, setHistoryList] = useState<CRMBroadcastSession[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState('')
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)
  const [sessionDetail, setSessionDetail] = useState<CRMBroadcastSession | null>(null)
  const [sessionLogs, setSessionLogs] = useState<CRMBroadcastLog[]>([])
  const [detailLoading, setDetailLoading] = useState(false)
  const [recipientFilter, setRecipientFilter] = useState<'all' | 'sent' | 'failed'>('all')

  const fetchHistory = async () => {
    setHistoryLoading(true)
    setHistoryError('')
    try {
      const res = await apiFetch<{ broadcasts: CRMBroadcastSession[] }>('/api/v1/contacts/broadcast/history')
      setHistoryList(res.broadcasts || [])
    } catch (err) {
      setHistoryError(err instanceof Error ? err.message : t('Gagal memuat riwayat broadcast'))
    } finally {
      setHistoryLoading(false)
    }
  }

  // Load history list initially
  useEffect(() => {
    fetchHistory()
  }, [])

  // Fetch session details when a session is selected
  useEffect(() => {
    if (!selectedSessionId) {
      setSessionDetail(null)
      setSessionLogs([])
      setRecipientFilter('all')
      return
    }

    let active = true
    setDetailLoading(true)
    apiFetch<{ broadcast: CRMBroadcastSession; logs: CRMBroadcastLog[] }>(
      `/api/v1/contacts/broadcast/history/${selectedSessionId}`,
    )
      .then((res) => {
        if (active) {
          setSessionDetail(res.broadcast)
          setSessionLogs(res.logs || res.broadcast?.logs || [])
        }
      })
      .catch((_err) => {
        if (active) {
          const found = historyList.find((s) => s.id === selectedSessionId)
          if (found) {
            setSessionDetail(found)
            setSessionLogs(found.logs || [])
          }
        }
      })
      .finally(() => {
        if (active) setDetailLoading(false)
      })

    return () => {
      active = false
    }
  }, [selectedSessionId, historyList])

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
    setSubject('')
    setContent('')
    setSendSuccess(null)
    setSendError('')
    idempotencyKeyRef.current = crypto.randomUUID()
  }

  const handleSendBroadcast = async () => {
    if (sending || !audience || audience.total_unique === 0) return
    const trimmedSubject = subject.trim()
    const trimmedContent = content.trim()
    if (!trimmedSubject || !trimmedContent) return

    setSending(true)
    setSendError('')

    try {
      const res = await apiFetch<BroadcastSendResponse>('/api/v1/contacts/broadcast/send', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tags: selectedTags,
          custom_emails: customEmails,
          subject: trimmedSubject,
          content: trimmedContent,
          idempotency_key: idempotencyKeyRef.current,
        }),
      })
      setSendSuccess(res)
      setSendError('')
      fetchHistory()
    } catch (err) {
      setSendError(err instanceof Error ? err.message : t('Gagal mengirim broadcast email'))
    } finally {
      setSending(false)
    }
  }

  const handleNewDraft = () => {
    setSubject('')
    setContent('')
    setSendSuccess(null)
    setSendError('')
    idempotencyKeyRef.current = crypto.randomUUID()
  }

  const filteredLogs = sessionLogs.filter((l) => {
    if (recipientFilter === 'sent') return l.status === 'sent'
    if (recipientFilter === 'failed') return l.status === 'failed'
    return true
  })

  const canSend =
    !sending &&
    !sendSuccess &&
    !!audience &&
    audience.total_unique > 0 &&
    subject.trim().length > 0 &&
    content.trim().length > 0

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
            {(selectedTags.length > 0 || customEmails.trim() || subject || content || sendSuccess) && (
              <button
                type="button"
                onClick={clearAll}
                className="action-secondary text-xs px-2.5 py-1 min-h-[32px]"
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

      {/* Sub-tab Navigation: Tulis Broadcast vs Riwayat Pengiriman (Things 3 / Bear Notes style) */}
      <div className="flex items-center gap-1 border-b border-border/40 pb-3" role="tablist" aria-label={t('Navigasi Broadcast')}>
        <button
          type="button"
          role="tab"
          id="broadcast-tab-compose"
          aria-selected={activeTab === 'compose'}
          aria-controls="broadcast-panel-compose"
          onClick={() => setActiveTab('compose')}
          className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
            activeTab === 'compose'
              ? 'bg-primary/10 text-primary font-semibold'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
          }`}
        >
          <PenLine className="h-3.5 w-3.5" />
          <span>{t('Tulis Broadcast')}</span>
        </button>

        <button
          type="button"
          role="tab"
          id="broadcast-tab-history"
          aria-selected={activeTab === 'history'}
          aria-controls="broadcast-panel-history"
          onClick={() => {
            setActiveTab('history')
            fetchHistory()
          }}
          className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
            activeTab === 'history'
              ? 'bg-primary/10 text-primary font-semibold'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
          }`}
        >
          <History className="h-3.5 w-3.5" />
          <span>{t('Riwayat Pengiriman')}</span>
          {historyList.length > 0 && (
            <span className="rounded-full bg-muted px-1.5 py-0.2 text-[10px] tabular-nums text-muted-foreground font-mono">
              {historyList.length}
            </span>
          )}
        </button>
      </div>

      {activeTab === 'compose' ? (
        /* Main Form & Preview Canvas */
        <div id="broadcast-panel-compose" role="tabpanel" aria-labelledby="broadcast-tab-compose" className="space-y-6">
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

            <Textarea
              id="custom-emails-textarea"
              aria-label={t('Alamat email kustom')}
              rows={3}
              value={customEmails}
              onChange={(e) => setCustomEmails(e.target.value)}
              placeholder="misal: investor@example.com, partner@example.com"
              mono
              className="min-h-[72px]"
            />
          </section>

          {/* Error notification for audience */}
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

                <div className="divide-y divide-border/30 rounded-lg border border-border/40 overflow-hidden bg-card/30 max-h-56 overflow-y-auto">
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

          {/* Section 4: Message Composer */}
          <section aria-labelledby="message-composer-heading" className="border-t border-border/40 pt-5 space-y-3">
            <div className="flex items-center justify-between">
              <h3
                id="message-composer-heading"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5"
              >
                <PenLine className="h-3.5 w-3.5" />
                <span>{t('Draf Pesan Broadcast')}</span>
              </h3>
              <span className="text-[11px] text-muted-foreground">
                {t('Editor Pesan')}
              </span>
            </div>

            <div className="space-y-4">
              <FormField label={t('Subjek Pesan')} htmlFor="broadcast-subject-input">
                <Input
                  id="broadcast-subject-input"
                  type="text"
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  placeholder={t('misal: Update Rilis Fitur & Berita Penting')}
                  disabled={sending || !!sendSuccess}
                />
              </FormField>

              <FormField label={t('Isi Pesan')} htmlFor="broadcast-content-textarea">
                <Textarea
                  id="broadcast-content-textarea"
                  rows={6}
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  placeholder={t('Tulis isi pengumuman broadcast di sini…')}
                  disabled={sending || !!sendSuccess}
                  className="min-h-[140px]"
                />
              </FormField>

              {/* Quiet RFC 8058 notice */}
              <div className="flex items-center gap-2 text-[11px] text-muted-foreground pt-1">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
                <span>
                  {t(
                    'Header List-Unsubscribe (RFC 8058) dan link unsubscribe standar disematkan otomatis pada setiap email.',
                  )}
                </span>
              </div>
            </div>
          </section>

          {/* Section 5: Send Action & Live Status Indicator */}
          <section aria-labelledby="send-action-heading" className="border-t border-border/40 pt-5 space-y-4">
            {/* Sending live indicator */}
            {sending && (
              <div
                role="status"
                className="flex items-center gap-2 rounded-lg border border-primary/30 bg-primary/5 p-3.5 text-xs text-foreground"
              >
                <Loader2 className="h-4 w-4 animate-spin text-primary shrink-0" />
                <div className="flex-1">
                  <p className="font-medium text-primary">
                    {t('Sedang mengirim broadcast…')}
                  </p>
                  <p className="text-[11px] text-muted-foreground mt-0.5">
                    {t('Memproses pengiriman batch ke')} {audience?.total_unique || 0} {t('penerima. Mohon tunggu…')}
                  </p>
                </div>
              </div>
            )}

            {/* Success status card */}
            {sendSuccess && (
              <div
                role="status"
                className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-4 space-y-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-2 text-emerald-600 dark:text-emerald-400 font-medium text-xs">
                    <Check className="h-4 w-4 shrink-0" />
                    <span>
                      {t('Sukses terkirim ke')} {sendSuccess.sent_count} {t('penerima')}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => {
                        setActiveTab('history')
                        fetchHistory()
                        if (sendSuccess.broadcast_id) {
                          setSelectedSessionId(sendSuccess.broadcast_id)
                        }
                      }}
                      className="action-secondary text-xs px-2.5 py-1 min-h-[28px]"
                    >
                      {t('Lihat Log & Riwayat')}
                    </button>
                    <button
                      type="button"
                      onClick={handleNewDraft}
                      className="action-secondary text-xs px-2.5 py-1 min-h-[28px]"
                    >
                      {t('Tulis Broadcast Baru')}
                    </button>
                  </div>
                </div>

                <p className="text-xs text-muted-foreground">
                  {sendSuccess.message}
                </p>

                {sendSuccess.already_sent && (
                  <p className="text-[11px] text-muted-foreground italic">
                    {t('Permintaan pengiriman ini telah diproses sebelumnya (terlindungi idempotency).')}
                  </p>
                )}

                {sendSuccess.failed_count > 0 && sendSuccess.errors && (
                  <div className="rounded border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-700 dark:text-amber-300 space-y-1">
                    <p className="font-medium">
                      {t('Perhatian:')} {sendSuccess.failed_count} {t('email gagal dikirim:')}
                    </p>
                    <ul className="list-disc list-inside space-y-0.5 text-[11px]">
                      {sendSuccess.errors.slice(0, 5).map((err, i) => (
                        <li key={i}>{err}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}

            {/* Error notification */}
            {sendError && (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive"
              >
                <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
                <div className="flex-1 space-y-1">
                  <p className="font-medium">{t('Gagal mengirim broadcast')}</p>
                  <p className="text-[11px] opacity-90">{t(sendError)}</p>
                </div>
              </div>
            )}

            {/* Action button row */}
            {!sendSuccess && (
              <div className="flex flex-wrap items-center justify-between gap-4 pt-1">
                <div className="text-[11px] text-muted-foreground">
                  {!audience || audience.total_unique === 0 ? (
                    <span>{t('Pilih minimal 1 penerima untuk mengaktifkan pengiriman.')}</span>
                  ) : !subject.trim() || !content.trim() ? (
                    <span>{t('Lengkapi subjek dan isi pesan sebelum mengirim.')}</span>
                  ) : (
                    <span className="text-emerald-600 dark:text-emerald-400">
                      {t('Siap dikirim ke')} {audience.total_unique} {t('penerima unik.')}
                    </span>
                  )}
                </div>

                <button
                  id="send-broadcast-button"
                  type="button"
                  onClick={handleSendBroadcast}
                  disabled={!canSend}
                  className="action-primary text-xs px-4 py-2 font-medium min-h-10 disabled:opacity-40"
                >
                  {sending ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      <span>{t('Sedang mengirim…')}</span>
                    </>
                  ) : (
                    <>
                      <Send className="h-3.5 w-3.5" />
                      <span>
                        {t('Kirim Broadcast ke')} {audience?.total_unique || 0} {t('Penerima')}
                      </span>
                    </>
                  )}
                </button>
              </div>
            )}
          </section>
        </div>
      ) : (
        /* History & Delivery Log List View (DoD 3) */
        <div id="broadcast-panel-history" role="tabpanel" aria-labelledby="broadcast-tab-history" className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
              <History className="h-3.5 w-3.5" />
              <span>{t('Daftar Riwayat Pengiriman')}</span>
            </h3>
            <span className="text-[11px] text-muted-foreground">
              {historyList.length} {t('sesi')}
            </span>
          </div>

          {historyLoading ? (
            <div className="rounded-lg border border-border/40 p-8 text-center bg-muted/5 space-y-2">
              <Loader2 className="h-4 w-4 animate-spin mx-auto text-primary" />
              <p className="text-xs text-muted-foreground">{t('Memuat riwayat broadcast…')}</p>
            </div>
          ) : historyError ? (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive"
            >
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              <p className="flex-1">{historyError}</p>
            </div>
          ) : historyList.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/60 p-8 text-center bg-muted/5 space-y-3">
              <History className="h-6 w-6 mx-auto text-muted-foreground/50" />
              <p className="text-xs text-muted-foreground">
                {t('Belum ada riwayat broadcast email yang pernah dikirim.')}
              </p>
              <button
                type="button"
                onClick={() => setActiveTab('compose')}
                className="action-secondary text-xs px-3 py-1.5 min-h-[32px]"
              >
                {t('Tulis Broadcast Baru')}
              </button>
            </div>
          ) : (
            <div className="space-y-2">
              {historyList.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedSessionId(item.id)}
                  className="w-full text-left p-3.5 rounded-lg border border-border/40 bg-card/40 hover:bg-muted/30 transition-colors space-y-2 group cursor-pointer block"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1 space-y-1">
                      <h4 className="font-medium text-xs sm:text-sm text-foreground truncate group-hover:text-primary transition-colors">
                        {item.subject}
                      </h4>
                      <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          <span>{formatTimestamp(item.created_at, language)}</span>
                        </span>
                        <span>·</span>
                        <span>
                          {item.total_recipients} {t('penerima')} ({item.sent_count} {t('terkirim')}
                          {item.failed_count > 0 ? `, ${item.failed_count} ${t('gagal')}` : ''})
                        </span>
                      </div>
                    </div>

                    <div className="shrink-0 flex items-center gap-2">
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium border ${
                          item.status === 'success'
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                            : item.status === 'partial'
                            ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
                            : 'bg-destructive/10 text-destructive border-destructive/20'
                        }`}
                      >
                        {item.status === 'success'
                          ? t('Terkirim')
                          : item.status === 'partial'
                          ? t('Sebagian')
                          : t('Gagal')}
                      </span>
                      <ChevronRight className="h-4 w-4 text-muted-foreground/60 group-hover:text-foreground transition-colors" />
                    </div>
                  </div>

                  {item.tags && item.tags.length > 0 && (
                    <div className="flex flex-wrap gap-1 pt-0.5">
                      {item.tags.map((tg) => (
                        <span
                          key={tg}
                          className="inline-flex items-center rounded-full bg-muted/60 px-2 py-0.5 text-[10px] font-mono border border-border/40 text-muted-foreground"
                        >
                          #{tg.replace(/^#/, '')}
                        </span>
                      ))}
                    </div>
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Detail View / SlideDrawer for Sesi Broadcast (DoD 4) */}
      <SlideDrawer
        isOpen={!!selectedSessionId}
        onClose={() => setSelectedSessionId(null)}
        title={sessionDetail?.subject || t('Rincian Sesi Broadcast')}
        description={sessionDetail ? formatTimestamp(sessionDetail.created_at, language) : ''}
        size="lg"
        data-testid="broadcast-history-detail-drawer"
      >
        <div className="space-y-6 p-1">
          {detailLoading ? (
            <div className="py-12 text-center space-y-2">
              <Loader2 className="h-5 w-5 animate-spin mx-auto text-primary" />
              <p className="text-xs text-muted-foreground">{t('Memuat rincian sesi…')}</p>
            </div>
          ) : sessionDetail ? (
            <div className="space-y-6">
              {/* Status and Metric Cards */}
              <div className="grid grid-cols-3 gap-2 text-center">
                <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-1">
                  <p className="text-[11px] text-muted-foreground font-medium uppercase tracking-wider">
                    {t('Total Penerima')}
                  </p>
                  <p className="text-lg font-semibold tabular-nums text-foreground font-mono">
                    {sessionDetail.total_recipients}
                  </p>
                </div>

                <div className="rounded-lg bg-emerald-500/5 border border-emerald-500/20 p-3 space-y-1">
                  <p className="text-[11px] text-emerald-600 dark:text-emerald-400 font-medium uppercase tracking-wider">
                    {t('Berhasil')}
                  </p>
                  <p className="text-lg font-semibold tabular-nums text-emerald-600 dark:text-emerald-400 font-mono">
                    {sessionDetail.sent_count}
                  </p>
                </div>

                <div className={`rounded-lg p-3 space-y-1 border ${
                  sessionDetail.failed_count > 0
                    ? 'bg-destructive/5 border-destructive/20 text-destructive'
                    : 'bg-muted/30 border-border/40 text-muted-foreground'
                }`}>
                  <p className="text-[11px] font-medium uppercase tracking-wider">
                    {t('Gagal')}
                  </p>
                  <p className={`text-lg font-semibold tabular-nums font-mono ${
                    sessionDetail.failed_count > 0 ? 'text-destructive' : 'text-foreground'
                  }`}>
                    {sessionDetail.failed_count}
                  </p>
                </div>
              </div>

              {/* Status Banner */}
              <div
                className={`flex items-center gap-2 rounded-lg p-3 text-xs border ${
                  sessionDetail.status === 'success'
                    ? 'bg-emerald-500/5 border-emerald-500/20 text-emerald-700 dark:text-emerald-300'
                    : sessionDetail.status === 'partial'
                    ? 'bg-amber-500/10 border-amber-500/20 text-amber-700 dark:text-amber-300'
                    : 'bg-destructive/10 border-destructive/20 text-destructive'
                }`}
              >
                {sessionDetail.status === 'success' ? (
                  <Check className="h-4 w-4 shrink-0" />
                ) : (
                  <AlertCircle className="h-4 w-4 shrink-0" />
                )}
                <span className="font-medium">{sessionDetail.message}</span>
              </div>

              {/* Metadata target (tags, custom emails) */}
              <div className="space-y-2 border-t border-border/40 pt-4">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('Target Penerima')}
                </h4>
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  {sessionDetail.tags && sessionDetail.tags.length > 0 ? (
                    sessionDetail.tags.map((tg) => (
                      <span
                        key={tg}
                        className="inline-flex items-center rounded-full bg-muted/60 px-2.5 py-0.5 text-[11px] font-mono border border-border/40 text-muted-foreground"
                      >
                        #{tg.replace(/^#/, '')}
                      </span>
                    ))
                  ) : (
                    <span className="text-xs text-muted-foreground italic">
                      {t('Tidak menggunakan segmen tag CRM')}
                    </span>
                  )}
                  {sessionDetail.custom_emails && sessionDetail.custom_emails.length > 0 && (
                    <span className="text-[11px] text-muted-foreground">
                      · {sessionDetail.custom_emails.length} {t('email kustom')}
                    </span>
                  )}
                </div>
              </div>

              {/* Message Content */}
              <div className="space-y-2 border-t border-border/40 pt-4">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('Subjek & Isi Pesan')}
                </h4>
                <div className="rounded-lg bg-muted/20 border border-border/30 p-3.5 space-y-2">
                  <p className="text-xs font-semibold text-foreground">
                    {sessionDetail.subject}
                  </p>
                  <p className="text-xs text-foreground/90 font-sans leading-relaxed whitespace-pre-wrap max-h-48 overflow-y-auto">
                    {sessionDetail.content}
                  </p>
                </div>
              </div>

              {/* Delivery logs for each recipient */}
              <div className="space-y-3 border-t border-border/40 pt-4">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('Status Tiap Penerima')}
                  </h4>
                  <div className="flex items-center gap-1 text-[11px]">
                    <button
                      type="button"
                      onClick={() => setRecipientFilter('all')}
                      className={`px-2.5 py-0.5 rounded-full border transition-colors ${
                        recipientFilter === 'all'
                          ? 'bg-primary text-primary-foreground border-primary font-medium'
                          : 'bg-muted/40 text-muted-foreground border-border/40 hover:bg-muted/70'
                      }`}
                    >
                      {t('Semua')} ({sessionLogs.length})
                    </button>
                    <button
                      type="button"
                      onClick={() => setRecipientFilter('sent')}
                      className={`px-2.5 py-0.5 rounded-full border transition-colors ${
                        recipientFilter === 'sent'
                          ? 'bg-emerald-600 text-white border-emerald-600 font-medium'
                          : 'bg-muted/40 text-muted-foreground border-border/40 hover:bg-muted/70'
                      }`}
                    >
                      {t('Sukses')} ({sessionLogs.filter((l) => l.status === 'sent').length})
                    </button>
                    {sessionLogs.some((l) => l.status === 'failed') && (
                      <button
                        type="button"
                        onClick={() => setRecipientFilter('failed')}
                        className={`px-2.5 py-0.5 rounded-full border transition-colors ${
                          recipientFilter === 'failed'
                            ? 'bg-destructive text-destructive-foreground border-destructive font-medium'
                            : 'bg-muted/40 text-destructive border-border/40 hover:bg-muted/70'
                        }`}
                      >
                        {t('Gagal')} ({sessionLogs.filter((l) => l.status === 'failed').length})
                      </button>
                    )}
                  </div>
                </div>

                <div className="divide-y divide-border/30 rounded-lg border border-border/40 overflow-hidden bg-card/30 max-h-64 overflow-y-auto">
                  {filteredLogs.length === 0 ? (
                    <p className="p-4 text-center text-xs text-muted-foreground">
                      {t('Tidak ada log penerima untuk filter yang dipilih.')}
                    </p>
                  ) : (
                    filteredLogs.map((log) => (
                      <div key={log.id} className="p-3 text-xs space-y-1">
                        <div className="flex items-center justify-between gap-2">
                          <div className="min-w-0 flex-1">
                            <p className="font-medium text-foreground truncate">
                              {log.name || log.email}
                            </p>
                            {log.name && log.name !== log.email && (
                              <p className="font-mono text-[11px] text-muted-foreground truncate">
                                {log.email}
                              </p>
                            )}
                          </div>
                          <div className="flex items-center gap-2 shrink-0">
                            {log.source === 'crm' && (
                              <span className="text-[10px] text-muted-foreground px-1.5 py-0.5 rounded bg-muted/60 border border-border/40 font-mono">
                                CRM
                              </span>
                            )}
                            {log.source === 'custom' && (
                              <span className="text-[10px] text-blue-600 dark:text-blue-400 px-1.5 py-0.5 rounded bg-blue-500/10 border border-blue-500/20 font-medium">
                                {t('Kustom')}
                              </span>
                            )}
                            {log.source === 'both' && (
                              <span className="text-[10px] text-purple-600 dark:text-purple-400 px-1.5 py-0.5 rounded bg-purple-500/10 border border-purple-500/20 font-medium">
                                {t('CRM + Kustom')}
                              </span>
                            )}
                            {log.status === 'sent' ? (
                              <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                                <Check className="h-3.5 w-3.5" />
                                <span>{t('Terkirim')}</span>
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-[11px] font-medium text-destructive">
                                <AlertCircle className="h-3.5 w-3.5" />
                                <span>{t('Gagal')}</span>
                              </span>
                            )}
                          </div>
                        </div>
                        {log.status === 'failed' && log.error_message && (
                          <div className="rounded bg-destructive/10 border border-destructive/20 p-2 text-[11px] text-destructive font-mono break-all mt-1">
                            {log.error_message}
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          ) : (
            <p className="py-6 text-center text-xs text-muted-foreground">
              {t('Sesi broadcast tidak ditemukan.')}
            </p>
          )}
        </div>
      </SlideDrawer>
    </aside>
  )
}
