import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
import { useState, useEffect, useMemo, useCallback } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Brain,
  Sparkles,
  Terminal,
  Plus,
  Pencil,
  Trash2,
  RotateCcw,
  Check,
  AlertTriangle,
  Info,
  X,
  ArrowLeft,
  ArrowRight,
  ShieldCheck,
  FileText,
  Clock,
  Sliders,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import {
  tasteApi,
  type TasteItem,
  type TasteProfile,
  type TasteCategory,
  type TasteCandidate,
  type TasteQuotaResponse,
  type TasteConfig,
} from '../../api/taste'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { Badge } from '../../components/ui/badge'

export interface AgentMemoryViewProps {
  showHeading?: boolean
  embedded?: boolean
  returnTo?: string
}

export function AgentMemoryView({
  showHeading = true,
  embedded = false,
  returnTo,
}: AgentMemoryViewProps) {
  const { t } = useLanguage()
  const location = useLocation()

  // Safe back navigation
  const query = new URLSearchParams(location.search)
  const resolvedReturnTo =
    returnTo ||
    (query.get('returnTo') === '/chat' || query.get('returnTo') === '/studio'
      ? '/chat'
      : query.get('returnTo') || '')

  // View switch: 'active' vs 'suggested'
  const [activeView, setActiveView] = useState<'active' | 'suggested'>(() => {
    const tParam = query.get('tab') || query.get('view')
    return tParam === 'suggested' || tParam === 'pending' ? 'suggested' : 'active'
  })

  // Data state
  const [profile, setProfile] = useState<TasteProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // Hermes Quota & Auto-Approve state (Sprint 45 S45-T3)
  const [quota, setQuota] = useState<TasteQuotaResponse | null>(null)
  const [config, setConfig] = useState<TasteConfig | null>(null)
  const [configSaving, setConfigSaving] = useState(false)
  const [autoApproveToast, setAutoApproveToast] = useState<{
    id: string
    text: string
    confidence?: number
    category: string
  } | null>(null)
  const [undoingCandidateId, setUndoingCandidateId] = useState<string | null>(null)

  // Candidates / Suggested Taste state (Sprint 44 & 45)
  const [candidates, setCandidates] = useState<TasteCandidate[]>([])
  const [candidatesLoading, setCandidatesLoading] = useState(false)
  const [reflecting, setReflecting] = useState(false)
  const [editingCandidate, setEditingCandidate] = useState<TasteCandidate | null>(null)
  const [candidateActionBusyId, setCandidateActionBusyId] = useState<string | null>(null)

  // Filter & Search
  const [selectedCategory, setSelectedCategory] = useState<
    'all' | 'brand' | 'technical'
  >('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Dialog states for active items
  const [addModalOpen, setAddModalOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<TasteItem | null>(null)
  const [deleteConfirmItem, setDeleteConfirmItem] = useState<TasteItem | null>(null)
  const [resetModalOpen, setResetModalOpen] = useState(false)

  // Form states
  const [formCategory, setFormCategory] = useState<TasteCategory>('brand')
  const [formTopic, setFormTopic] = useState('')
  const [formContent, setFormContent] = useState('')
  const [formSaving, setFormSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [actionBusy, setActionBusy] = useState(false)

  // Load Profile
  const loadProfile = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await tasteApi.getProfile()
      setProfile(data)
    } catch (err: any) {
      setError(
        err?.message ||
          t('Failed to load agent memory profile. Please try again.'),
      )
    } finally {
      setLoading(false)
    }
  }, [t])

  // Load Candidates
  const loadCandidates = useCallback(async () => {
    try {
      setCandidatesLoading(true)
      const data = await tasteApi.getCandidates({ status: 'pending' })
      setCandidates(data.candidates || data.items || [])
    } catch (err: any) {
      console.error('Failed to load taste candidates:', err)
    } finally {
      setCandidatesLoading(false)
    }
  }, [])

  // Load Quota
  const loadQuota = useCallback(async () => {
    try {
      const q = await tasteApi.getQuota()
      setQuota(q)
    } catch (err: any) {
      console.error('Failed to load taste quota:', err)
    }
  }, [])

  // Load Config
  const loadConfig = useCallback(async () => {
    try {
      const cfg = await tasteApi.getConfig()
      setConfig(cfg)
    } catch (err: any) {
      console.error('Failed to load taste config:', err)
    }
  }, [])

  useEffect(() => {
    void loadProfile()
    void loadCandidates()
    void loadQuota()
    void loadConfig()
  }, [loadProfile, loadCandidates, loadQuota, loadConfig])

  // Clear feedback timer
  useEffect(() => {
    if (!feedback) return
    const timer = setTimeout(() => {
      setFeedback(null)
    }, 4000)
    return () => clearTimeout(timer)
  }, [feedback])

  // Auto-Approve Toggle Handler
  const handleToggleAutoApprove = async () => {
    const nextVal = !(config ? config.auto_approve_enabled : (quota ? quota.auto_approve_enabled : true))
    try {
      setConfigSaving(true)
      const updated = await tasteApi.updateConfig({
        auto_approve_enabled: nextVal,
      })
      setConfig(updated)
      await loadQuota()
      setFeedback({
        type: 'success',
        message: updated.auto_approve_enabled
          ? t('Smart Auto-Approval activated.')
          : t('Smart Auto-Approval deactivated.'),
      })
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to update auto-approve settings'),
      })
    } finally {
      setConfigSaving(false)
    }
  }

  // Threshold Update Handler
  const handleUpdateThreshold = async (newThreshold: number) => {
    try {
      setConfigSaving(true)
      const updated = await tasteApi.updateConfig({
        auto_approve_threshold: newThreshold,
      })
      setConfig(updated)
      await loadQuota()
      setFeedback({
        type: 'success',
        message: `${t('Confidence Threshold updated to')} ${Math.round(newThreshold * 100)}%.`,
      })
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to update threshold'),
      })
    } finally {
      setConfigSaving(false)
    }
  }

  // Undo Auto-Approve Handler
  const handleUndoCandidate = async (candidateId: string) => {
    try {
      setUndoingCandidateId(candidateId)
      await tasteApi.undoCandidate(candidateId)
      setAutoApproveToast(null)
      setFeedback({
        type: 'success',
        message: t('Auto-approval undone, preference returned to suggested list.'),
      })
      await loadProfile()
      await loadCandidates()
      await loadQuota()
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to undo auto-approval'),
      })
    } finally {
      setUndoingCandidateId(null)
    }
  }

  // Open Add Modal
  const handleOpenAdd = (category?: TasteCategory) => {
    setFormCategory(category || (selectedCategory === 'technical' ? 'technical' : 'brand'))
    setFormTopic('')
    setFormContent('')
    setFormError(null)
    setAddModalOpen(true)
  }

  // Open Edit Modal for active items
  const handleOpenEdit = (item: TasteItem) => {
    setEditingItem(item)
    setFormCategory(item.category)
    setFormTopic(item.topic || '')
    setFormContent(item.content)
    setFormError(null)
  }

  // Submit Add
  const handleSaveNew = async (e: React.FormEvent) => {
    e.preventDefault()
    const content = formContent.trim()
    if (!content) {
      setFormError(t('Preference content is required'))
      return
    }
    if (content.length > 2000) {
      setFormError(t('Preference content cannot exceed 2000 characters'))
      return
    }
    if (formTopic.trim().length > 100) {
      setFormError(t('Topic cannot exceed 100 characters'))
      return
    }

    try {
      setFormSaving(true)
      setFormError(null)
      await tasteApi.createItem({
        category: formCategory,
        topic: formTopic.trim() || undefined,
        content,
      })
      setAddModalOpen(false)
      setFeedback({
        type: 'success',
        message: t('New memory preference saved successfully.'),
      })
      await loadProfile()
      await loadQuota()
    } catch (err: any) {
      setFormError(err?.message || t('Failed to save preference'))
    } finally {
      setFormSaving(false)
    }
  }

  // Submit Edit for active item
  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingItem) return
    const content = formContent.trim()
    if (!content) {
      setFormError(t('Preference content is required'))
      return
    }
    if (content.length > 2000) {
      setFormError(t('Preference content cannot exceed 2000 characters'))
      return
    }
    if (formTopic.trim().length > 100) {
      setFormError(t('Topic cannot exceed 100 characters'))
      return
    }

    try {
      setFormSaving(true)
      setFormError(null)
      await tasteApi.updateItem(editingItem.id, {
        category: formCategory,
        topic: formTopic.trim() || undefined,
        content,
      })
      setEditingItem(null)
      setFeedback({
        type: 'success',
        message: t('Preference updated successfully.'),
      })
      await loadProfile()
      await loadQuota()
    } catch (err: any) {
      setFormError(err?.message || t('Failed to update preference'))
    } finally {
      setFormSaving(false)
    }
  }

  // Delete / Forget Item
  const handleConfirmDelete = async () => {
    if (!deleteConfirmItem) return
    try {
      setActionBusy(true)
      await tasteApi.deleteItem(deleteConfirmItem.id)
      setDeleteConfirmItem(null)
      setFeedback({
        type: 'success',
        message: t('Preference forgotten successfully.'),
      })
      await loadProfile()
      await loadQuota()
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to forget preference'),
      })
    } finally {
      setActionBusy(false)
    }
  }

  // Reset Profile
  const handleConfirmReset = async () => {
    try {
      setActionBusy(true)
      const reset = await tasteApi.resetProfile()
      setProfile(reset)
      setResetModalOpen(false)
      setFeedback({
        type: 'success',
        message: t('All agent memory reset to default templates.'),
      })
      await loadQuota()
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to reset memory'),
      })
    } finally {
      setActionBusy(false)
    }
  }

  // Candidate Actions (Accept / Dismiss / Reflect)
  const handleAcceptCandidate = async (
    candidate: TasteCandidate,
    edits?: { category?: TasteCategory; topic?: string; content?: string; text?: string },
  ) => {
    try {
      setCandidateActionBusyId(candidate.id)
      await tasteApi.acceptCandidate(candidate.id, {
        category: edits?.category,
        topic: edits?.topic,
        content: edits?.content || edits?.text,
        text: edits?.text || edits?.content,
      })
      setCandidates((prev) => prev.filter((c) => c.id !== candidate.id))
      setEditingCandidate(null)
      setFeedback({
        type: 'success',
        message: t('Candidate preference approved and promoted to active memory.'),
      })
      await loadProfile()
      await loadCandidates()
      await loadQuota()
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to approve candidate'),
      })
    } finally {
      setCandidateActionBusyId(null)
    }
  }

  const handleDismissCandidate = async (candidate: TasteCandidate) => {
    try {
      setCandidateActionBusyId(candidate.id)
      await tasteApi.dismissCandidate(candidate.id)
      setCandidates((prev) => prev.filter((c) => c.id !== candidate.id))
      setFeedback({
        type: 'success',
        message: t('Candidate preference dismissed.'),
      })
      await loadCandidates()
      await loadQuota()
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to dismiss candidate'),
      })
    } finally {
      setCandidateActionBusyId(null)
    }
  }

  const handleTriggerReflection = async () => {
    try {
      setReflecting(true)
      const res = await tasteApi.reflectTaste()
      await loadCandidates()
      await loadProfile()
      await loadQuota()
      if (res.auto_approved && res.auto_approved.length > 0) {
        const first = res.auto_approved[0]
        setAutoApproveToast({
          id: first.id,
          text: first.text || first.content || '',
          confidence: first.confidence,
          category: first.category,
        })
        setFeedback({
          type: 'success',
          message: `${t('Reflection completed successfully.')} ${res.auto_approved.length} ${t('preference(s) auto-approved')}`,
        })
      } else if (res.count > 0 || (res.candidates && res.candidates.length > 0)) {
        setFeedback({
          type: 'success',
          message: `${t('Reflection completed successfully.')} ${res.count || res.candidates.length} ${t('pending suggestion(s)')}`,
        })
      } else {
        setFeedback({
          type: 'success',
          message: t('No new preferences detected in recent conversations.'),
        })
      }
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to run reflection'),
      })
    } finally {
      setReflecting(false)
    }
  }

  const handleOpenEditCandidate = (candidate: TasteCandidate) => {
    setEditingCandidate(candidate)
    setFormCategory(candidate.category)
    setFormTopic(candidate.topic || '')
    setFormContent(candidate.text || candidate.content || '')
    setFormError(null)
  }

  const handleSaveEditCandidate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingCandidate) return
    const content = formContent.trim()
    if (!content) {
      setFormError(t('Preference content is required'))
      return
    }
    if (content.length > 2000) {
      setFormError(t('Preference content cannot exceed 2000 characters'))
      return
    }
    if (formTopic.trim().length > 100) {
      setFormError(t('Topic cannot exceed 100 characters'))
      return
    }

    try {
      setFormSaving(true)
      setFormError(null)
      await handleAcceptCandidate(editingCandidate, {
        category: formCategory,
        topic: formTopic.trim() || undefined,
        content,
      })
    } catch (err: any) {
      setFormError(err?.message || t('Failed to approve candidate'))
    } finally {
      setFormSaving(false)
    }
  }

  // Items filtering (Active Memory)
  const filteredItems = useMemo(() => {
    if (!profile) return []
    let list = profile.items || []
    if (selectedCategory === 'brand') {
      list = profile.brand_items || list.filter((i) => i.category === 'brand')
    } else if (selectedCategory === 'technical') {
      list =
        profile.technical_items ||
        list.filter((i) => i.category === 'technical')
    }

    if (!searchQuery.trim()) return list
    const q = searchQuery.toLowerCase()
    return list.filter(
      (item) =>
        (item.topic && item.topic.toLowerCase().includes(q)) ||
        item.content.toLowerCase().includes(q) ||
        item.category.toLowerCase().includes(q),
    )
  }, [profile, selectedCategory, searchQuery])

  // Candidates filtering (Suggested Taste)
  const filteredCandidates = useMemo(() => {
    let list = candidates || []
    if (selectedCategory === 'brand') {
      list = list.filter((c) => c.category === 'brand')
    } else if (selectedCategory === 'technical') {
      list = list.filter((c) => c.category === 'technical')
    }

    if (!searchQuery.trim()) return list
    const q = searchQuery.toLowerCase()
    return list.filter(
      (c) =>
        (c.topic && c.topic.toLowerCase().includes(q)) ||
        (c.text && c.text.toLowerCase().includes(q)) ||
        (c.content && c.content.toLowerCase().includes(q)) ||
        (c.reason && c.reason.toLowerCase().includes(q)) ||
        (c.rationale && c.rationale.toLowerCase().includes(q)) ||
        c.category.toLowerCase().includes(q),
    )
  }, [candidates, selectedCategory, searchQuery])

  // Quota and Counts calculations
  const brandCount = profile?.brand_items?.length ?? 0
  const technicalCount = profile?.technical_items?.length ?? 0
  const totalCount = profile?.total ?? (profile?.items?.length ?? 0)
  const pendingCount = candidates.length

  const brandUsed = quota?.brand.used ?? brandCount
  const brandMax = quota?.brand.max ?? 15
  const brandPct = quota?.brand.percentage ?? (brandMax > 0 ? Math.round((brandUsed / brandMax) * 100) : 0)
  const brandWarning = brandUsed >= 12
  const brandFull = brandUsed >= brandMax

  const techUsed = quota?.technical.used ?? technicalCount
  const techMax = quota?.technical.max ?? 15
  const techPct = quota?.technical.percentage ?? (techMax > 0 ? Math.round((techUsed / techMax) * 100) : 0)
  const techWarning = techUsed >= 12
  const techFull = techUsed >= techMax

  const totalUsed = quota?.total_used ?? (brandUsed + techUsed)
  const totalMax = quota?.total_max ?? 30
  const totalPct = totalMax > 0 ? Math.round((totalUsed / totalMax) * 100) : 0

  const autoApproveEnabled = config?.auto_approve_enabled ?? quota?.auto_approve_enabled ?? true
  const autoApproveThreshold = config?.auto_approve_threshold ?? quota?.auto_approve_threshold ?? 0.8
  const thresholdPct = Math.round(autoApproveThreshold * 100)

  return (
    <div className={embedded ? 'space-y-4' : 'space-y-6'}>
      {/* Return Navigation if coming from Chat */}
      {resolvedReturnTo && (
        <div>
          <Link
            to={resolvedReturnTo}
            className="action-secondary inline-flex min-h-11 items-center gap-2 text-sm"
          >
            <ArrowLeft className="h-4 w-4" />
            {t(
              resolvedReturnTo === '/chat' || resolvedReturnTo === '/studio'
                ? 'Back to Chat'
                : 'Back',
            )}
          </Link>
        </div>
      )}

      {/* Auto-Approve Toast Notification with 1-Click Undo (Sprint 45 S45-T3) */}
      {autoApproveToast && (
        <aside
          role="status"
          aria-live="polite"
          data-testid="auto-approve-toast"
          className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-4 text-emerald-950 dark:text-emerald-100 shadow-sm"
        >
          <div className="flex items-start sm:items-center gap-3 min-w-0">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-500/20 text-emerald-600 dark:text-emerald-400">
              <Sparkles className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-semibold text-foreground text-sm">
                  {t('Preferensi baru otomatis disetujui')}
                </span>
                {autoApproveToast.confidence !== undefined && (
                  <span
                    data-testid="toast-confidence-badge"
                    className="inline-flex items-center gap-1 rounded-full bg-emerald-500/20 text-emerald-800 dark:text-emerald-200 border border-emerald-500/40 px-2 py-0.5 text-[10px] font-mono font-semibold"
                  >
                    {Math.round(autoApproveToast.confidence * 100)}% {t('Keyakinan')}
                  </span>
                )}
              </div>
              <p className="text-xs text-foreground/85 mt-0.5 truncate max-w-xl">
                "{autoApproveToast.text}"
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0 self-start sm:self-auto">
            <button
              type="button"
              data-testid="undo-auto-approve-btn"
              onClick={() => void handleUndoCandidate(autoApproveToast.id)}
              disabled={undoingCandidateId === autoApproveToast.id}
              className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3.5 text-xs font-semibold text-foreground hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors"
            >
              <RotateCcw
                className={`h-3.5 w-3.5 ${
                  undoingCandidateId === autoApproveToast.id ? 'animate-spin' : ''
                }`}
              />
              <span>{t('Batalkan')} (Undo)</span>
            </button>
            <button
              type="button"
              onClick={() => setAutoApproveToast(null)}
              className="icon-action text-muted-foreground hover:text-foreground"
              aria-label={t('Dismiss')}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </aside>
      )}

      {/* Header and Hero Overview */}
      <section className="space-y-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          {showHeading && <div className="flex items-start gap-3.5">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Brain className="h-6 w-6" />
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-xl font-bold tracking-tight text-foreground">
                  {t('Agent Memory / Taste')}
                </h2>
                <Badge variant="outline" className="text-xs font-mono">
                  {t('Self-Improving')}
                </Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground leading-relaxed">
                {t(
                  'Learned preferences and guidelines that agents apply automatically to understand your brand voice and workflows without repetitive prompts.',
                )}
              </p>
            </div>
          </div>}

          {/* Quick Header Actions */}
          <div className="flex flex-wrap items-center gap-2.5 sm:self-center">
            <button
              type="button"
              onClick={() => handleOpenAdd()}
              className="action-primary min-h-11 inline-flex items-center gap-1.5 px-3.5 text-sm font-medium"
            >
              <Plus className="h-4 w-4" />
              <span>{t('Add Preference')}</span>
            </button>
       </div>
        </div>

      </section>

      <details className="border-y border-border/40 py-2">
        <summary className="min-h-11 cursor-pointer py-3 text-sm text-muted-foreground">{t('Learning rules and memory capacity')}</summary>
        <div className="space-y-6 py-4">
            <button
              type="button"
              onClick={() => setResetModalOpen(true)}
              className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3 text-sm text-muted-foreground hover:text-destructive"
              title={t('Reset all memory back to default templates')}
            >
              <RotateCcw className="h-4 w-4" />
              <span>{t('Reset All Memory')}</span>
            </button>

        {/* Quick Summary Pill Bar */}
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4 border-t border-border/60 pt-4 text-xs">
          <div className="flex items-center gap-2.5 rounded-lg bg-muted/40 p-2.5">
            <Sparkles className="h-4 w-4 text-purple-500 shrink-0" />
            <div>
              <span className="font-semibold text-foreground">
                {t('Brand & Voice')}
              </span>
              <p className="text-muted-foreground">
                {brandCount} / 15 {t('points active')}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5 rounded-lg bg-muted/40 p-2.5">
            <Terminal className="h-4 w-4 text-sky-500 shrink-0" />
            <div>
              <span className="font-semibold text-foreground">
                {t('Technical & Workflows')}
              </span>
              <p className="text-muted-foreground">
                {technicalCount} / 15 {t('points active')}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5 rounded-lg bg-muted/40 p-2.5">
            <ShieldCheck className="h-4 w-4 text-emerald-500 shrink-0" />
            <div>
              <span className="font-semibold text-foreground">
                {t('Context Window Guard')}
              </span>
              <p className="text-muted-foreground">
                {t('15 item limit prevents token bloat')}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={() => setActiveView('suggested')}
            className={`flex items-center gap-2.5 rounded-lg p-2.5 text-left transition-colors ${
              activeView === 'suggested'
                ? 'bg-amber-500/15 border border-amber-500/30'
                : 'bg-muted/40 hover:bg-muted/70'
            }`}
          >
            <Clock className="h-4 w-4 text-amber-500 shrink-0" />
            <div>
              <span className="font-semibold text-foreground">
                {t('Suggested Taste')}
              </span>
              <p className="text-muted-foreground">
                {pendingCount} {t('pending suggestion(s)')}
              </p>
            </div>
          </button>
        </div>

      {/* Hermes-Style Memory Quota Meter (Sprint 45 S45-T3) */}
      <section
        data-testid="hermes-quota-meter"
        aria-label={t('Hermes Memory Quota Meter')}
        className="border-b border-border/40 pb-5 space-y-4"
      >
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 border-b border-border/60 pb-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ShieldCheck className="h-4 w-4" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-bold text-foreground">
                  {t('Hermes Memory Quota Meter')}
                </h3>
                <span className="rounded-full bg-primary/10 text-primary border border-primary/20 px-2 py-0.5 text-[10px] font-mono font-semibold">
                  15 {t('items / category')}
                </span>
              </div>
              <p className="text-xs text-muted-foreground">
                {t('Hermes Memory Quota prevents token bloat by limiting memory to 15 items per category.')}
              </p>
            </div>
          </div>
          <div className="text-xs font-mono font-semibold text-muted-foreground self-start sm:self-auto">
            {t('Total Memori')}: <span className="text-foreground">{totalUsed} / {totalMax}</span> ({totalPct}%)
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Brand & Voice Quota Meter */}
          <div
            data-testid="quota-meter-brand"
            className={`rounded-xl border p-4 space-y-3 transition-colors ${
              brandFull
                ? 'border-rose-500/40 bg-rose-500/5'
                : brandWarning
                ? 'border-amber-500/40 bg-amber-500/5'
                : 'border-border bg-muted/20'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Sparkles className="h-4 w-4 text-purple-500 shrink-0" />
                <span className="text-xs font-bold text-foreground">
                  {t('Brand & Voice')}
                </span>
              </div>
              {brandFull ? (
                <span
                  data-testid="quota-badge-brand"
                  className="rounded-full bg-rose-500/20 text-rose-800 dark:text-rose-200 border border-rose-500/40 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Quota Full')} (15/15)
                </span>
              ) : brandWarning ? (
                <span
                  data-testid="quota-badge-brand"
                  className="rounded-full bg-amber-500/20 text-amber-800 dark:text-amber-200 border border-amber-500/40 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Approaching Limit')}
                </span>
              ) : (
                <span
                  data-testid="quota-badge-brand"
                  className="rounded-full bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Optimal Capacity')}
                </span>
              )}
            </div>

            {/* Quota Counter Indicator e.g. '8/15 Butir Terpakai (53%)' */}
            <div className="flex items-baseline justify-between text-xs">
              <span
                data-testid="quota-count-brand"
                className="font-mono font-semibold text-foreground text-sm"
              >
                {brandUsed}/{brandMax} {t('Butir Terpakai')} ({brandPct}%)
              </span>
              <span className="text-[11px] text-muted-foreground">
                {brandMax - brandUsed} {t('tersisa')}
              </span>
            </div>

            {/* Progress Bar Track */}
            <div
              className="h-2.5 w-full rounded-full bg-muted overflow-hidden"
              role="progressbar"
              aria-valuenow={brandUsed}
              aria-valuemin={0}
              aria-valuemax={brandMax}
            >
              <div
                data-testid="quota-bar-brand"
                style={{ width: `${Math.min(100, Math.max(0, brandPct))}%` }}
                className={`h-full rounded-full transition-all duration-500 ${
                  brandFull
                    ? 'bg-rose-500'
                    : brandWarning
                    ? 'bg-amber-500'
                    : 'bg-purple-600'
                }`}
              />
            </div>

            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {brandFull
                ? t('Quota full (15/15) - auto-approvals paused and held for manual review.')
                : brandWarning
                ? t('Approaching quota limit (>= 12 items).')
                : t('Tersisa kapasitas memori untuk preferensi gaya komunikasi & brand voice.')}
            </p>
          </div>

          {/* Technical & Workflows Quota Meter */}
          <div
            data-testid="quota-meter-technical"
            className={`rounded-xl border p-4 space-y-3 transition-colors ${
              techFull
                ? 'border-rose-500/40 bg-rose-500/5'
                : techWarning
                ? 'border-amber-500/40 bg-amber-500/5'
                : 'border-border bg-muted/20'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Terminal className="h-4 w-4 text-sky-500 shrink-0" />
                <span className="text-xs font-bold text-foreground">
                  {t('Technical & Workflows')}
                </span>
              </div>
              {techFull ? (
                <span
                  data-testid="quota-badge-technical"
                  className="rounded-full bg-rose-500/20 text-rose-800 dark:text-rose-200 border border-rose-500/40 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Quota Full')} (15/15)
                </span>
              ) : techWarning ? (
                <span
                  data-testid="quota-badge-technical"
                  className="rounded-full bg-amber-500/20 text-amber-800 dark:text-amber-200 border border-amber-500/40 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Approaching Limit')}
                </span>
              ) : (
                <span
                  data-testid="quota-badge-technical"
                  className="rounded-full bg-sky-500/15 text-sky-700 dark:text-sky-300 border border-sky-500/30 px-2 py-0.5 text-[10px] font-semibold"
                >
                  {t('Optimal Capacity')}
                </span>
              )}
            </div>

            {/* Quota Counter Indicator e.g. '8/15 Butir Terpakai (53%)' */}
            <div className="flex items-baseline justify-between text-xs">
              <span
                data-testid="quota-count-technical"
                className="font-mono font-semibold text-foreground text-sm"
              >
                {techUsed}/{techMax} {t('Butir Terpakai')} ({techPct}%)
              </span>
              <span className="text-[11px] text-muted-foreground">
                {techMax - techUsed} {t('tersisa')}
              </span>
            </div>

            {/* Progress Bar Track */}
            <div
              className="h-2.5 w-full rounded-full bg-muted overflow-hidden"
              role="progressbar"
              aria-valuenow={techUsed}
              aria-valuemin={0}
              aria-valuemax={techMax}
            >
              <div
                data-testid="quota-bar-technical"
                style={{ width: `${Math.min(100, Math.max(0, techPct))}%` }}
                className={`h-full rounded-full transition-all duration-500 ${
                  techFull
                    ? 'bg-rose-500'
                    : techWarning
                    ? 'bg-amber-500'
                    : 'bg-sky-600'
                }`}
              />
            </div>

            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {techFull
                ? t('Quota full (15/15) - auto-approvals paused and held for manual review.')
                : techWarning
                ? t('Approaching quota limit (>= 12 items).')
                : t('Tersisa kapasitas memori untuk preferensi alur kerja & aturan teknis.')}
            </p>
          </div>
        </div>
      </section>

      {/* Smart Auto-Approval Controls Card (Sprint 45 S45-T3) */}
      <section
        data-testid="auto-approve-controls"
        aria-label={t('Auto-Approval Controls')}
        className="border-b border-border/40 pb-5 space-y-4"
      >
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div className="flex items-start gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-bold text-foreground">
                  {t('Smart Auto-Approval')}
                </h3>
                <span
                  data-testid="auto-approve-status-pill"
                  className={`rounded-full px-2 py-0.5 text-[10px] font-semibold border ${
                    autoApproveEnabled
                      ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30'
                      : 'bg-muted text-muted-foreground border-border'
                  }`}
                >
                  {autoApproveEnabled ? t('Auto-Approve Enabled') : t('Auto-Approve Disabled')}
                </span>
              </div>
              <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">
                {t('Preferences with confidence >= threshold are automatically approved when quota allows (< 15 items).')}
              </p>
            </div>
          </div>

          {/* Toggle Switch */}
          <button
            type="button"
            role="switch"
            aria-checked={autoApproveEnabled}
            data-testid="auto-approve-toggle"
            onClick={handleToggleAutoApprove}
            disabled={configSaving}
            className={`min-h-11 inline-flex items-center gap-2.5 rounded-lg px-4 text-xs font-semibold transition-all shrink-0 self-start sm:self-auto ${
              autoApproveEnabled
                ? 'bg-emerald-600 hover:bg-emerald-700 text-white shadow-sm'
                : 'bg-muted hover:bg-muted/80 text-foreground border border-border'
            }`}
          >
            {autoApproveEnabled ? (
              <>
                <Check className="h-4 w-4" />
                <span>{t('Auto-Approve Aktif')}</span>
              </>
            ) : (
              <>
                <X className="h-4 w-4 text-muted-foreground" />
                <span>{t('Auto-Approve Nonaktif')}</span>
              </>
            )}
          </button>
        </div>

        {/* Threshold Slider and Settings (when enabled) */}
        {autoApproveEnabled && (
          <div
            data-testid="auto-approve-threshold-container"
            className="rounded-lg border border-border/80 bg-muted/30 p-4 space-y-3"
          >
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1">
              <label
                htmlFor="threshold-slider"
                className="text-xs font-bold text-foreground flex items-center gap-2"
              >
                <Sliders className="h-3.5 w-3.5 text-primary" />
                <span>{t('Ambang Batas Keyakinan')} ({t('Confidence Threshold')}):</span>
                <span
                  data-testid="threshold-value-display"
                  className="font-mono text-primary text-sm font-bold bg-primary/10 px-2 py-0.5 rounded"
                >
                  {thresholdPct}%
                </span>
              </label>
              <span className="text-[11px] text-muted-foreground">
                {t('Keyakinan')} &gt;= {thresholdPct}% {t('langsung dipromosikan otomatis')}
              </span>
            </div>

            {/* Slider Input */}
            <div className="space-y-1.5">
              <input
                id="threshold-slider"
                type="range"
                min="50"
                max="95"
                step="5"
                value={thresholdPct}
                data-testid="auto-approve-threshold-slider"
                disabled={configSaving}
                onChange={(e) => void handleUpdateThreshold(Number(e.target.value) / 100)}
                className="w-full h-2 bg-muted rounded-lg appearance-none cursor-pointer accent-primary"
              />
              <div className="flex justify-between text-[10px] text-muted-foreground font-mono px-0.5">
                <span>50%</span>
                <span>65%</span>
                <span>80% ({t('Standar')})</span>
                <span>90%</span>
                <span>95%</span>
              </div>
            </div>

            {/* Quick Presets */}
            <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-border/40 text-xs">
              <span className="text-[11px] text-muted-foreground">{t('Pilihan Cepat')}:</span>
              {[0.75, 0.8, 0.85, 0.9].map((p) => {
                const pct = Math.round(p * 100)
                const isSelected = thresholdPct === pct
                return (
                  <button
                    key={p}
                    type="button"
                    onClick={() => void handleUpdateThreshold(p)}
                    disabled={configSaving}
                    className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                      isSelected
                        ? 'bg-primary text-primary-foreground font-bold'
                        : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground'
                    }`}
                  >
                    {pct}% {pct === 80 ? `(${t('Standar')})` : ''}
                  </button>
                )
              })}
            </div>
          </div>
        )}
      </section>

        </div>
      </details>

      {/* Global Toast / Feedback Alert */}
      {feedback && (
        <div
          role="status"
          aria-live="polite"
          className={`flex items-center justify-between rounded-lg p-3 text-sm transition-all ${
            feedback.type === 'success'
              ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30'
              : 'bg-destructive/10 text-destructive border border-destructive/30'
          }`}
        >
          <div className="flex items-center gap-2">
            {feedback.type === 'success' ? (
              <Check className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
            ) : (
              <AlertTriangle className="h-4 w-4 shrink-0 text-destructive" />
            )}
            <span>{feedback.message}</span>
          </div>
          <button
            type="button"
            onClick={() => setFeedback(null)}
            className="icon-action text-muted-foreground hover:text-foreground"
            aria-label={t('Dismiss')}
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Primary Sub-Navigation Tabs: Active Memory vs Suggested Taste */}
      <ListTabs aria-label={t('Memory view mode')}>
        <ListTab selected={activeView === 'active'} count={totalCount} onClick={() => setActiveView('active')}>{t('Active Memory')}</ListTab>
        <ListTab selected={activeView === 'suggested'} count={pendingCount} onClick={() => setActiveView('suggested')}><span data-testid="suggested-tab-counter">{t('Suggested preferences')}</span></ListTab>
      </ListTabs>

      {/* Suggested Taste Prompt Banner when in Active view */}
      {activeView === 'active' && pendingCount > 0 && (
        <aside
          aria-label={t('Suggested Preferences')}
          className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-900 dark:text-amber-100"
        >
          <div className="flex items-start sm:items-center gap-2.5">
            <Sparkles className="h-5 w-5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5 sm:mt-0" />
            <div>
              <p className="font-semibold text-foreground">
                {t('There are suggested preferences awaiting your review.')} ({pendingCount})
              </p>
              <p className="text-muted-foreground mt-0.5">
                {t('Review suggested preferences extracted from conversations before promoting them to active memory.')}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setActiveView('suggested')}
            className="action-primary min-h-11 shrink-0 px-4 text-xs inline-flex items-center gap-1.5 self-start sm:self-auto"
          >
            <span>{t('Review Suggestions')}</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </button>
        </aside>
      )}

      {/* Suggested Taste Header Explanatory Box when in Suggested view */}
      {activeView === 'suggested' && (
        <section className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-4 text-xs">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="flex items-start gap-2.5">
              <Sparkles className="h-5 w-5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5" />
              <div>
                <p className="font-semibold text-foreground text-sm">
                  {t('Suggested Preferences')} ({t('Autonomous Reflection')})
                </p>
                <p className="text-muted-foreground mt-1 leading-relaxed">
                  {t('Agent automatically extracts new preferences from your chat conversations. You can also trigger reflection on demand.')}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={handleTriggerReflection}
              disabled={reflecting}
              className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3.5 text-xs font-semibold shrink-0"
              title={t('Trigger autonomous reflection loop')}
            >
              {reflecting ? (
                <>
                  <RotateCcw className="h-3.5 w-3.5 animate-spin text-amber-600" />
                  <span>{t('Reflecting…')}</span>
                </>
              ) : (
                <>
                  <Sparkles className="h-3.5 w-3.5 text-amber-600" />
                  <span>{t('Reflect Now')}</span>
                </>
              )}
            </button>
          </div>
        </section>
      )}

      <section className="space-y-3">
        <ListSearch aria-label={t('Search memory')} placeholder={t('Search memory…')} value={searchQuery} onChange={event => setSearchQuery(event.target.value)} />
        <ListTabs aria-label={t('Filter memory categories')}>
          <ListTab selected={selectedCategory === 'all'} onClick={() => setSelectedCategory('all')}>{t('All Memory')}</ListTab>
          <ListTab selected={selectedCategory === 'brand'} onClick={() => setSelectedCategory('brand')}>{t('Brand & Voice')}</ListTab>
          <ListTab selected={selectedCategory === 'technical'} onClick={() => setSelectedCategory('technical')}>{t('Technical & Workflows')}</ListTab>
        </ListTabs>
      </section>

      {/* ======================= ACTIVE MEMORY VIEW ======================= */}
      {activeView === 'active' && (
        <>
          {/* Loading State */}
          {loading && (
            <div
              role="status"
              className="flex flex-col items-center justify-center p-12 text-center text-muted-foreground"
            >
              <Brain className="h-8 w-8 animate-pulse text-primary mb-3" />
              <p className="text-sm">{t('Loading agent memory preferences…')}</p>
            </div>
          )}

          {/* Error State */}
          {!loading && error && (
            <div
              role="alert"
              className="rounded-xl border border-destructive/30 bg-destructive/5 p-5 text-center text-destructive"
            >
              <AlertTriangle className="mx-auto h-7 w-7 mb-2" />
              <p className="text-sm font-semibold">{error}</p>
              <button
                type="button"
                onClick={() => void loadProfile()}
                className="action-secondary mt-3 min-h-11 text-xs"
              >
                {t('Try again')}
              </button>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && filteredItems.length === 0 && (
            <div className="rounded-xl border border-dashed border-border bg-card/50 p-8 text-center">
              <FileText className="mx-auto h-9 w-9 text-muted-foreground/60 mb-2" />
              <h3 className="text-base font-semibold text-foreground">
                {searchQuery
                  ? t('No matching preferences found')
                  : t('No memory preferences recorded yet')}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                {searchQuery
                  ? t('Try adjusting your search query or switch categories.')
                  : t(
                      'Add brand guidelines or technical rules to teach your agent how to work according to your taste.',
                    )}
              </p>
              <div className="mt-4 flex flex-wrap justify-center gap-2">
                <button
                  type="button"
                  onClick={() => handleOpenAdd()}
                  className="action-primary min-h-11 inline-flex items-center gap-1.5 px-3.5 text-xs font-medium"
                >
                  <Plus className="h-4 w-4" />
                  <span>{t('Add First Preference')}</span>
                </button>
                <button
                  type="button"
                  onClick={() => setResetModalOpen(true)}
                  className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3 text-xs"
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  <span>{t('Load Standard Defaults')}</span>
                </button>
              </div>
            </div>
          )}

          {/* Active Preference Cards Grid */}
          {!loading && !error && filteredItems.length > 0 && (
            <div className="divide-y divide-border/40">
              {filteredItems.map((item) => {
                const isBrand = item.category === 'brand'
                return (
                  <article
                    key={item.id}
                    data-testid={`active-taste-card-${item.id}`}
                    className="group relative flex flex-col justify-between py-5"
                  >
                    <div>
                      {/* Category Header & Topic */}
                      <div className="flex items-start justify-between gap-2 mb-2.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span
                            className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold ${
                              isBrand
                                ? 'bg-transparent text-muted-foreground dark:text-muted-foreground border border-transparent'
                                : 'bg-transparent text-muted-foreground dark:text-muted-foreground border border-transparent'
                            }`}
                          >
                            {isBrand ? (
                              <Sparkles className="h-3 w-3 shrink-0" />
                            ) : (
                              <Terminal className="h-3 w-3 shrink-0" />
                            )}
                            <span>
                              {isBrand
                                ? t('Brand & Voice')
                                : t('Technical & Workflow')}
                            </span>
                          </span>

                          {/* Auto-Approved Badge */}
                          {item.auto_approved && (
                            <span
                              data-testid={`auto-approved-badge-${item.id}`}
                              className="inline-flex items-center gap-1 rounded-full bg-transparent text-muted-foreground dark:text-muted-foreground border border-transparent px-2 py-0.5 text-[10px] font-semibold"
                              title={t('Preferensi ini otomatis disetujui oleh Smart Auto-Approval')}
                            >
                              <Sparkles className="h-2.5 w-2.5 text-muted-foreground dark:text-muted-foreground shrink-0" />
                              <span>{t('Otomatis Disetujui')}</span>
                            </span>
                          )}

                          {item.topic && (
                            <span className="font-semibold text-xs text-foreground bg-muted/60 px-2 py-0.5 rounded-md line-clamp-1">
                              {item.topic}
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Preference Content Body */}
                      <p className="text-sm text-foreground/90 whitespace-pre-wrap leading-relaxed break-words">
                        {item.content}
                      </p>
                    </div>

                    {/* Footer Actions & Timestamp */}
                    <div className="mt-4 flex items-center justify-between border-t border-border/40 pt-3 text-xs text-muted-foreground">
                      <span className="text-[11px]">
                        {item.updated_at
                          ? new Date(item.updated_at).toLocaleDateString(undefined, {
                              month: 'short',
                              day: 'numeric',
                              year: 'numeric',
                            })
                          : t('Default template')}
                      </span>

                      {/* Granular Edit & Forget Controls */}
                      <div className="flex items-center gap-1">
                        <button
                          type="button"
                          onClick={() => handleOpenEdit(item)}
                          aria-label={t('Edit preference')}
                          className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                          title={t('Edit preference')}
                        >
                          <Pencil className="h-4 w-4" />
                        </button>

                        <button
                          type="button"
                          onClick={() => setDeleteConfirmItem(item)}
                          aria-label={t('Lupakan poin ini')}
                          className="inline-flex min-h-11 items-center gap-1.5 rounded-lg px-2.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors text-xs font-medium"
                          title={t('Lupakan poin ini')}
                        >
                          <Trash2 className="h-4 w-4 shrink-0" />
                          <span>{t('Lupakan poin ini')}</span>
                        </button>
                      </div>
                    </div>
                  </article>
                )
              })}
            </div>
          )}
        </>
      )}

      {/* ===================== SUGGESTED TASTE VIEW ===================== */}
      {activeView === 'suggested' && (
        <section aria-label={t('Suggested Taste / Pending Reflection')}>
          {/* Loading Candidates */}
          {candidatesLoading && (
            <div
              role="status"
              className="flex flex-col items-center justify-center p-12 text-center text-muted-foreground"
            >
              <Sparkles className="h-8 w-8 animate-pulse text-amber-500 mb-3" />
              <p className="text-sm">{t('Loading suggested preferences…')}</p>
            </div>
          )}

          {/* Empty Candidates State */}
          {!candidatesLoading && filteredCandidates.length === 0 && (
            <div className="rounded-xl border border-dashed border-border bg-card/50 p-8 text-center">
              <Sparkles className="mx-auto h-9 w-9 text-amber-500/70 mb-2" />
              <h3 className="text-base font-semibold text-foreground">
                {searchQuery
                  ? t('No matching preferences found')
                  : t('No suggested preferences pending')}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground max-w-md mx-auto leading-relaxed">
                {searchQuery
                  ? t('Try adjusting your search query or switch categories.')
                  : t('Agent automatically extracts new preferences from your chat conversations. You can also trigger reflection on demand.')}
              </p>
              <div className="mt-4 flex justify-center">
                <button
                  type="button"
                  onClick={handleTriggerReflection}
                  disabled={reflecting}
                  className="action-primary min-h-11 inline-flex items-center gap-1.5 px-4 text-xs font-medium"
                >
                  {reflecting ? (
                    <>
                      <RotateCcw className="h-3.5 w-3.5 animate-spin" />
                      <span>{t('Reflecting…')}</span>
                    </>
                  ) : (
                    <>
                      <Sparkles className="h-3.5 w-3.5" />
                      <span>{t('Reflect Now')}</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          )}

          {/* Suggested Candidate Cards Grid */}
          {!candidatesLoading && filteredCandidates.length > 0 && (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {filteredCandidates.map((candidate) => {
                const isBrand = candidate.category === 'brand'
                const isBusy = candidateActionBusyId === candidate.id
                const rationale = candidate.reason || candidate.rationale || candidate.source_snippet
                const confValue = candidate.confidence !== undefined ? candidate.confidence : 0.85
                const confPercent = Math.round(confValue * 100)
                const isHighConfidence = confPercent >= 80

                return (
                  <article
                    key={candidate.id}
                    data-testid={`candidate-card-${candidate.id}`}
                    className="group relative flex flex-col justify-between rounded-xl border border-amber-500/30 bg-card p-4 shadow-sm transition-all hover:border-amber-500/60 hover:shadow-md"
                  >
                    <div>
                      {/* Header Badges & Topic */}
                      <div className="flex items-start justify-between gap-2 mb-2.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span
                            className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold ${
                              isBrand
                                ? 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30'
                                : 'bg-sky-500/15 text-sky-700 dark:text-sky-300 border border-sky-500/30'
                            }`}
                          >
                            {isBrand ? (
                              <Sparkles className="h-3 w-3 shrink-0" />
                            ) : (
                              <Terminal className="h-3 w-3 shrink-0" />
                            )}
                            <span>
                              {isBrand
                                ? t('Brand & Voice')
                                : t('Technical & Workflow')}
                            </span>
                          </span>

                          <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30 px-2 py-0.5 text-[10px] font-semibold">
                            <Clock className="h-2.5 w-2.5" />
                            {t('Pending Review')}
                          </span>

                          {/* Confidence Badge (Sprint 45 S45-T3) */}
                          <span
                            data-testid={`confidence-badge-${candidate.id}`}
                            className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold border ${
                              isHighConfidence
                                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30'
                                : 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border-amber-500/30'
                            }`}
                            title={isHighConfidence ? t('Keyakinan Tinggi (>= 80%)') : t('Keyakinan Sedang (< 80%)')}
                          >
                            <ShieldCheck className="h-2.5 w-2.5 shrink-0" />
                            <span>{t('Keyakinan')} {confPercent}%</span>
                          </span>

                          {candidate.topic && (
                            <span className="font-semibold text-xs text-foreground bg-muted/60 px-2 py-0.5 rounded-md line-clamp-1">
                              {candidate.topic}
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Candidate Preference Text */}
                      <p className="text-sm font-medium text-foreground whitespace-pre-wrap leading-relaxed break-words">
                        {candidate.text || candidate.content}
                      </p>

                      {/* Rationale / Source Context */}
                      {rationale && (
                        <div className="mt-3 rounded-lg bg-muted/50 p-2.5 text-xs text-muted-foreground border border-border/40 space-y-1">
                          <div className="flex items-center gap-1.5 font-semibold text-foreground/80 text-[11px]">
                            <Info className="h-3.5 w-3.5 text-primary shrink-0" />
                            <span>{t('Rationale')}</span>
                          </div>
                          <p className="text-[11px] leading-relaxed italic">
                            "{rationale}"
                          </p>
                        </div>
                      )}
                    </div>

                    {/* Human-in-the-Loop Review Controls */}
                    <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border/40 pt-3 text-xs">
                      <span className="text-[11px] text-muted-foreground">
                        {candidate.created_at
                          ? new Date(candidate.created_at).toLocaleDateString(undefined, {
                              month: 'short',
                              day: 'numeric',
                              year: 'numeric',
                            })
                          : t('Suggested recently')}
                      </span>

                      {/* Action buttons: Accept (Terima), Edit & Accept (Edit & Simpan), Dismiss (Abaikan) */}
                      <div className="flex items-center gap-1.5">
                        {/* Abaikan (Dismiss) */}
                        <button
                          type="button"
                          onClick={() => void handleDismissCandidate(candidate)}
                          disabled={isBusy}
                          aria-label={t('Abaikan')}
                          className="inline-flex min-h-11 items-center gap-1.5 rounded-lg px-2.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors text-xs font-medium"
                          title={t('Abaikan')}
                        >
                          <X className="h-4 w-4 shrink-0" />
                          <span>{t('Abaikan')}</span>
                        </button>

                        {/* Edit & Simpan (Edit & Accept) */}
                        <button
                          type="button"
                          onClick={() => handleOpenEditCandidate(candidate)}
                          disabled={isBusy}
                          aria-label={t('Edit & Simpan')}
                          className="action-secondary inline-flex min-h-11 items-center gap-1.5 rounded-lg px-3 text-xs font-medium"
                          title={t('Edit & Simpan')}
                        >
                          <Pencil className="h-3.5 w-3.5 shrink-0" />
                          <span>{t('Edit & Simpan')}</span>
                        </button>

                        {/* Terima (Accept) */}
                        <button
                          type="button"
                          onClick={() => void handleAcceptCandidate(candidate)}
                          disabled={isBusy}
                          aria-label={t('Terima')}
                          className="inline-flex min-h-11 items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white px-3.5 text-xs font-semibold shadow-sm transition-colors"
                          title={t('Terima')}
                        >
                          {isBusy ? (
                            <RotateCcw className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Check className="h-4 w-4 shrink-0" />
                          )}
                          <span>{t('Terima')}</span>
                        </button>
                      </div>
                    </div>
                  </article>
                )
              })}
            </div>
          )}
        </section>
      )}

      {/* ======================= DIALOGS ======================= */}

      {/* ADD PREFERENCE DIALOG (Active) */}
      <ActionDialog
        open={addModalOpen}
        title={t('Add Memory Preference')}
        onClose={() => {
          if (!formSaving) setAddModalOpen(false)
        }}
        busy={formSaving}
      >
        <form onSubmit={handleSaveNew} className="space-y-4 text-sm">
          {formError && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center gap-2"
            >
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>{formError}</span>
            </div>
          )}

          {/* Category Selector */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">
              {t('Category')}
            </label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setFormCategory('brand')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'brand'
                    ? 'border-purple-500 bg-purple-500/10 text-foreground ring-1 ring-purple-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Sparkles className="h-3.5 w-3.5 text-purple-500" />
                  <span>{t('Brand & Voice')}</span>
                </div>
                <span className="text-[11px] text-muted-foreground mt-1">
                  {t('Tone, copywriting style, persona, and audience.')}
                </span>
              </button>

              <button
                type="button"
                onClick={() => setFormCategory('technical')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'technical'
                    ? 'border-sky-500 bg-sky-500/10 text-foreground ring-1 ring-sky-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Terminal className="h-3.5 w-3.5 text-sky-500" />
                  <span>{t('Technical & Workflows')}</span>
                </div>
                <span className="text-[11px] text-muted-foreground mt-1">
                  {t('Stack conventions, deployment rules, and safety.')}
                </span>
              </button>
            </div>
          </div>

          {/* Topic Field (Optional) */}
          <div>
            <label
              htmlFor="add-topic"
              className="block text-xs font-semibold text-foreground mb-1"
            >
              {t('Topic / Headline (Optional)')}
            </label>
            <input
              id="add-topic"
              type="text"
              value={formTopic}
              onChange={(e) => setFormTopic(e.target.value)}
              placeholder={t('e.g. Tone of Voice, Deployment safety, Copywriting')}
              maxLength={100}
              className="field-control min-h-11 w-full text-sm"
              disabled={formSaving}
            />
            <p className="mt-1 text-[11px] text-muted-foreground">
              {t('A short label to easily identify this rule (max 100 chars).')}
            </p>
          </div>

          {/* Content Field (Mandatory) */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label
                htmlFor="add-content"
                className="block text-xs font-semibold text-foreground"
              >
                {t('Guideline & Learned Rule')} *
              </label>
              <span className="text-[11px] text-muted-foreground">
                {formContent.length} / 2000
              </span>
            </div>
            <textarea
              id="add-content"
              rows={4}
              value={formContent}
              onChange={(e) => setFormContent(e.target.value)}
              placeholder={t(
                'Describe how you want agents to behave or what rules they must follow…',
              )}
              maxLength={2000}
              required
              className="field-control w-full p-2.5 text-sm resize-y"
              disabled={formSaving}
            />
          </div>

          {/* Dialog Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              onClick={() => setAddModalOpen(false)}
              disabled={formSaving}
              className="action-secondary min-h-11 px-4 text-sm"
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              disabled={formSaving || !formContent.trim()}
              className="action-primary min-h-11 px-4 text-sm inline-flex items-center gap-1.5"
            >
              {formSaving ? (
                <span>{t('Saving…')}</span>
              ) : (
                <>
                  <Check className="h-4 w-4" />
                  <span>{t('Save Preference')}</span>
                </>
              )}
            </button>
          </div>
        </form>
      </ActionDialog>

      {/* EDIT PREFERENCE DIALOG (Active Item) */}
      <ActionDialog
        open={!!editingItem}
        title={t('Edit Preference')}
        onClose={() => {
          if (!formSaving) setEditingItem(null)
        }}
        busy={formSaving}
      >
        <form onSubmit={handleSaveEdit} className="space-y-4 text-sm">
          {formError && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center gap-2"
            >
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>{formError}</span>
            </div>
          )}

          {/* Category Selector */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">
              {t('Category')}
            </label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setFormCategory('brand')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'brand'
                    ? 'border-purple-500 bg-purple-500/10 text-foreground ring-1 ring-purple-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Sparkles className="h-3.5 w-3.5 text-purple-500" />
                  <span>{t('Brand & Voice')}</span>
                </div>
              </button>

              <button
                type="button"
                onClick={() => setFormCategory('technical')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'technical'
                    ? 'border-sky-500 bg-sky-500/10 text-foreground ring-1 ring-sky-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Terminal className="h-3.5 w-3.5 text-sky-500" />
                  <span>{t('Technical & Workflows')}</span>
                </div>
              </button>
            </div>
          </div>

          {/* Topic Field */}
          <div>
            <label
              htmlFor="edit-topic"
              className="block text-xs font-semibold text-foreground mb-1"
            >
              {t('Topic / Headline (Optional)')}
            </label>
            <input
              id="edit-topic"
              type="text"
              value={formTopic}
              onChange={(e) => setFormTopic(e.target.value)}
              placeholder={t('e.g. Tone of Voice')}
              maxLength={100}
              className="field-control min-h-11 w-full text-sm"
              disabled={formSaving}
            />
          </div>

          {/* Content Field */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label
                htmlFor="edit-content"
                className="block text-xs font-semibold text-foreground"
              >
                {t('Guideline & Learned Rule')} *
              </label>
              <span className="text-[11px] text-muted-foreground">
                {formContent.length} / 2000
              </span>
            </div>
            <textarea
              id="edit-content"
              rows={4}
              value={formContent}
              onChange={(e) => setFormContent(e.target.value)}
              maxLength={2000}
              required
              className="field-control w-full p-2.5 text-sm resize-y"
              disabled={formSaving}
            />
          </div>

          {/* Dialog Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              onClick={() => setEditingItem(null)}
              disabled={formSaving}
              className="action-secondary min-h-11 px-4 text-sm"
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              disabled={formSaving || !formContent.trim()}
              className="action-primary min-h-11 px-4 text-sm inline-flex items-center gap-1.5"
            >
              {formSaving ? (
                <span>{t('Updating…')}</span>
              ) : (
                <>
                  <Check className="h-4 w-4" />
                  <span>{t('Update Preference')}</span>
                </>
              )}
            </button>
          </div>
        </form>
      </ActionDialog>

      {/* EDIT CANDIDATE DIALOG (Review & Edit before Accept) */}
      <ActionDialog
        open={!!editingCandidate}
        title={t('Review and Edit Suggestion')}
        onClose={() => {
          if (!formSaving) setEditingCandidate(null)
        }}
        busy={formSaving}
      >
        <form onSubmit={handleSaveEditCandidate} className="space-y-4 text-sm">
          {formError && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center gap-2"
            >
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>{formError}</span>
            </div>
          )}

          {/* Original Context / Rationale Info */}
          {editingCandidate && (editingCandidate.reason || editingCandidate.rationale || editingCandidate.source_snippet) && (
            <div className="rounded-lg bg-muted/60 p-3 text-xs space-y-1 border border-border/40">
              <div className="flex items-center gap-1.5 font-semibold text-foreground/80">
                <Info className="h-3.5 w-3.5 text-primary shrink-0" />
                <span>{t('Rationale')}</span>
              </div>
              <p className="text-muted-foreground italic">
                "{editingCandidate.reason || editingCandidate.rationale || editingCandidate.source_snippet}"
              </p>
            </div>
          )}

          {/* Category Selector */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">
              {t('Category')}
            </label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setFormCategory('brand')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'brand'
                    ? 'border-purple-500 bg-purple-500/10 text-foreground ring-1 ring-purple-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Sparkles className="h-3.5 w-3.5 text-purple-500" />
                  <span>{t('Brand & Voice')}</span>
                </div>
              </button>

              <button
                type="button"
                onClick={() => setFormCategory('technical')}
                className={`flex flex-col items-start p-3 rounded-lg border text-left transition-all ${
                  formCategory === 'technical'
                    ? 'border-sky-500 bg-sky-500/10 text-foreground ring-1 ring-sky-500'
                    : 'border-border bg-card text-muted-foreground hover:border-border/80'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-xs text-foreground">
                  <Terminal className="h-3.5 w-3.5 text-sky-500" />
                  <span>{t('Technical & Workflows')}</span>
                </div>
              </button>
            </div>
          </div>

          {/* Topic Field */}
          <div>
            <label
              htmlFor="edit-cand-topic"
              className="block text-xs font-semibold text-foreground mb-1"
            >
              {t('Topic / Headline (Optional)')}
            </label>
            <input
              id="edit-cand-topic"
              type="text"
              value={formTopic}
              onChange={(e) => setFormTopic(e.target.value)}
              placeholder={t('e.g. Tone of Voice')}
              maxLength={100}
              className="field-control min-h-11 w-full text-sm"
              disabled={formSaving}
            />
          </div>

          {/* Content Field */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label
                htmlFor="edit-cand-content"
                className="block text-xs font-semibold text-foreground"
              >
                {t('Guideline & Learned Rule')} *
              </label>
              <span className="text-[11px] text-muted-foreground">
                {formContent.length} / 2000
              </span>
            </div>
            <textarea
              id="edit-cand-content"
              rows={4}
              value={formContent}
              onChange={(e) => setFormContent(e.target.value)}
              maxLength={2000}
              required
              className="field-control w-full p-2.5 text-sm resize-y"
              disabled={formSaving}
            />
          </div>

          {/* Dialog Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              onClick={() => setEditingCandidate(null)}
              disabled={formSaving}
              className="action-secondary min-h-11 px-4 text-sm"
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              disabled={formSaving || !formContent.trim()}
              className="action-primary min-h-11 px-4 text-sm inline-flex items-center gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
            >
              {formSaving ? (
                <span>{t('Saving…')}</span>
              ) : (
                <>
                  <Check className="h-4 w-4" />
                  <span>{t('Save & Accept')}</span>
                </>
              )}
            </button>
          </div>
        </form>
      </ActionDialog>

      {/* CONFIRM DELETE ("Lupakan poin ini") DIALOG */}
      <ActionDialog
        open={!!deleteConfirmItem}
        title={t('Lupakan Poin Memori Ini?')}
        onClose={() => {
          if (!actionBusy) setDeleteConfirmItem(null)
        }}
        busy={actionBusy}
      >
        <div className="space-y-4 text-sm">
          <p className="text-muted-foreground leading-relaxed">
            {t(
              'Agents will no longer consider this preference in future conversations and task sessions.',
            )}
          </p>

          {deleteConfirmItem && (
            <div className="rounded-lg bg-muted/60 p-3 text-xs space-y-1">
              {deleteConfirmItem.topic && (
                <div className="font-semibold text-foreground">
                  {deleteConfirmItem.topic}
                </div>
              )}
              <div className="text-muted-foreground line-clamp-3">
                {deleteConfirmItem.content}
              </div>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              onClick={() => setDeleteConfirmItem(null)}
              disabled={actionBusy}
              className="action-secondary min-h-11 px-4 text-sm"
            >
              {t('Cancel')}
            </button>
            <button
              type="button"
              onClick={handleConfirmDelete}
              disabled={actionBusy}
              className="action-destructive min-h-11 px-4 text-sm inline-flex items-center gap-1.5"
            >
              {actionBusy ? (
                <span>{t('Forgetting…')}</span>
              ) : (
                <>
                  <Trash2 className="h-4 w-4" />
                  <span>{t('Lupakan Poin Ini')}</span>
                </>
              )}
            </button>
          </div>
        </div>
      </ActionDialog>

      {/* CONFIRM RESET TOTAL ("Reset Semua Memori") SAFE ALERT DIALOG */}
      <ActionDialog
        open={resetModalOpen}
        title={t('Reset Semua Memori Agen?')}
        onClose={() => {
          if (!actionBusy) setResetModalOpen(false)
        }}
        busy={actionBusy}
      >
        <div className="space-y-4 text-sm">
          <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-amber-800 dark:text-amber-200 text-xs leading-relaxed">
            <AlertTriangle className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
            <div>
              <p className="font-semibold text-foreground">
                {t('Restore default memory templates')}
              </p>
              <p className="mt-1">
                {t(
                  'This will reset your agent preferences back to the standard Cuan templates. Any custom preferences you added will be removed.',
                )}
              </p>
            </div>
          </div>

          <p className="text-xs text-muted-foreground leading-relaxed">
            <Info className="inline h-3.5 w-3.5 mr-1 text-primary" />
            {t(
              'Your projects, chat messages, contacts, and task history are completely safe and will not be affected.',
            )}
          </p>

          <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
            <button
              type="button"
              onClick={() => setResetModalOpen(false)}
              disabled={actionBusy}
              className="action-secondary min-h-11 px-4 text-sm"
            >
              {t('Cancel')}
            </button>
            <button
              type="button"
              onClick={handleConfirmReset}
              disabled={actionBusy}
              className="action-destructive min-h-11 px-4 text-sm inline-flex items-center gap-1.5"
            >
              {actionBusy ? (
                <span>{t('Resetting…')}</span>
              ) : (
                <>
                  <RotateCcw className="h-4 w-4" />
                  <span>{t('Yes, Reset All Memory')}</span>
                </>
              )}
            </button>
          </div>
        </div>
      </ActionDialog>
    </div>
  )
}
