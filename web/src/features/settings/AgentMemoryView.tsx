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
  Search,
  Check,
  AlertTriangle,
  Info,
  X,
  ArrowLeft,
  ShieldCheck,
  Layers,
  FileText,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import {
  tasteApi,
  type TasteItem,
  type TasteProfile,
  type TasteCategory,
} from '../../api/taste'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { Badge } from '../../components/ui/badge'

export interface AgentMemoryViewProps {
  embedded?: boolean
  returnTo?: string
}

export function AgentMemoryView({
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

  // Data state
  const [profile, setProfile] = useState<TasteProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // Filter & Search
  const [selectedCategory, setSelectedCategory] = useState<
    'all' | 'brand' | 'technical'
  >('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Dialog states
  const [addModalOpen, setAddModalOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<TasteItem | null>(null)
  const [deleteConfirmItem, setDeleteConfirmItem] = useState<TasteItem | null>(
    null,
  )
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

  useEffect(() => {
    void loadProfile()
  }, [loadProfile])

  // Clear feedback timer
  useEffect(() => {
    if (!feedback) return
    const timer = setTimeout(() => {
      setFeedback(null)
    }, 4000)
    return () => clearTimeout(timer)
  }, [feedback])

  // Open Add Modal
  const handleOpenAdd = (category?: TasteCategory) => {
    setFormCategory(category || (selectedCategory === 'technical' ? 'technical' : 'brand'))
    setFormTopic('')
    setFormContent('')
    setFormError(null)
    setAddModalOpen(true)
  }

  // Open Edit Modal
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
    } catch (err: any) {
      setFormError(err?.message || t('Failed to save preference'))
    } finally {
      setFormSaving(false)
    }
  }

  // Submit Edit
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
    } catch (err: any) {
      setFeedback({
        type: 'error',
        message: err?.message || t('Failed to reset memory'),
      })
    } finally {
      setActionBusy(false)
    }
  }

  // Items filtering
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

  const brandCount = profile?.brand_items?.length ?? 0
  const technicalCount = profile?.technical_items?.length ?? 0
  const totalCount = profile?.total ?? (profile?.items?.length ?? 0)

  return (
    <div className={embedded ? "space-y-4" : "space-y-6"}>
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
                ? '← Back to Chat'
                : '← Back',
            )}
          </Link>
        </div>
      )}

      {/* Header and Hero Overview */}
      <section className="rounded-xl border border-border bg-card p-5 shadow-sm">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3.5">
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
          </div>

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
            <button
              type="button"
              onClick={() => setResetModalOpen(true)}
              className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3 text-sm text-muted-foreground hover:text-destructive"
              title={t('Reset all memory back to default templates')}
            >
              <RotateCcw className="h-4 w-4" />
              <span>{t('Reset All Memory')}</span>
            </button>
          </div>
        </div>

        {/* Layer Metrics & Context Safety Limit */}
        <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3 border-t border-border/60 pt-4 text-xs">
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
        </div>
      </section>

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

      {/* Filter Chips & Search Bar */}
      <section className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {/* Category Tabs */}
        <div
          role="tablist"
          aria-label={t('Filter memory categories')}
          className="flex flex-wrap gap-2"
        >
          <button
            type="button"
            role="tab"
            aria-selected={selectedCategory === 'all'}
            onClick={() => setSelectedCategory('all')}
            className={`min-h-11 inline-flex items-center gap-1.5 rounded-lg px-3.5 text-xs font-medium transition-colors ${
              selectedCategory === 'all'
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground'
            }`}
          >
            <Layers className="h-3.5 w-3.5" />
            <span>{t('All Memory')}</span>
            <span className="ml-1 rounded-full bg-primary-foreground/20 px-1.5 py-0.2 text-[10px] font-mono">
              {totalCount}
            </span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={selectedCategory === 'brand'}
            onClick={() => setSelectedCategory('brand')}
            className={`min-h-11 inline-flex items-center gap-1.5 rounded-lg px-3.5 text-xs font-medium transition-colors ${
              selectedCategory === 'brand'
                ? 'bg-purple-600 text-white shadow-sm'
                : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground'
            }`}
          >
            <Sparkles className="h-3.5 w-3.5" />
            <span>{t('Brand Voice & Persona')}</span>
            <span className="ml-1 rounded-full bg-white/20 px-1.5 py-0.2 text-[10px] font-mono">
              {brandCount}
            </span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={selectedCategory === 'technical'}
            onClick={() => setSelectedCategory('technical')}
            className={`min-h-11 inline-flex items-center gap-1.5 rounded-lg px-3.5 text-xs font-medium transition-colors ${
              selectedCategory === 'technical'
                ? 'bg-sky-600 text-white shadow-sm'
                : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground'
            }`}
          >
            <Terminal className="h-3.5 w-3.5" />
            <span>{t('Technical & Workflows')}</span>
            <span className="ml-1 rounded-full bg-white/20 px-1.5 py-0.2 text-[10px] font-mono">
              {technicalCount}
            </span>
          </button>
        </div>

        {/* Search Input */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t('Search preferences…')}
            className="field-control min-h-11 w-full pl-9 pr-8 text-xs"
          />
          {searchQuery && (
            <button
              type="button"
              onClick={() => setSearchQuery('')}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label={t('Clear search')}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </section>

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

      {/* Preference Cards Grid */}
      {!loading && !error && filteredItems.length > 0 && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {filteredItems.map((item) => {
            const isBrand = item.category === 'brand'
            return (
              <article
                key={item.id}
                className="group relative flex flex-col justify-between rounded-xl border border-border bg-card p-4 transition-all hover:border-border/80 hover:shadow-sm"
              >
                <div>
                  {/* Category Header & Topic */}
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

                  {/* Granular Edit & Forget Controls (Touch standard min 44x44px) */}
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

      {/* ADD PREFERENCE DIALOG */}
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

      {/* EDIT PREFERENCE DIALOG */}
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
