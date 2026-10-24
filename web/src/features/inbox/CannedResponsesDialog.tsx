import { useState, useEffect, useCallback } from 'react'
import { Plus, Search, Trash2, Edit2, Check, AlertCircle } from 'lucide-react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { useLanguage } from '../../context/LanguageContext'
import {
  chatApi,
  CannedResponse,
  CreateCannedResponseRequest,
} from '../../api/chat'

interface CannedResponsesDialogProps {
  open: boolean
  onClose: () => void
  onSelect?: (content: string) => void
}

export function CannedResponsesDialog({
  open,
  onClose,
  onSelect,
}: CannedResponsesDialogProps) {
  const { t } = useLanguage()
  const [items, setItems] = useState<CannedResponse[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('all')
  const [editingItem, setEditingItem] = useState<CannedResponse | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  // Form state
  const [formShortcut, setFormShortcut] = useState('')
  const [formTitle, setFormTitle] = useState('')
  const [formCategory, setFormCategory] = useState('general')
  const [formContent, setFormContent] = useState('')

  const loadResponses = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await chatApi.listCannedResponses({
        q: search,
        category: category !== 'all' ? category : undefined,
      })
      setItems(data)
    } catch (err: any) {
      setError(err?.message || t('Could not load canned responses.'))
    } finally {
      setLoading(false)
    }
  }, [search, category, t])

  useEffect(() => {
    if (open) {
      loadResponses()
      setIsCreating(false)
      setEditingItem(null)
    }
  }, [open, loadResponses])

  const handleStartCreate = () => {
    setFormShortcut('')
    setFormTitle('')
    setFormCategory('general')
    setFormContent('')
    setError('')
    setEditingItem(null)
    setIsCreating(true)
  }

  const handleStartEdit = (item: CannedResponse) => {
    setFormShortcut(item.shortcut)
    setFormTitle(item.title)
    setFormCategory(item.category || 'general')
    setFormContent(item.content)
    setError('')
    setIsCreating(false)
    setEditingItem(item)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    const cleanShortcut = formShortcut.trim().replace(/^\/+/, '')
    const cleanTitle = formTitle.trim()
    const cleanContent = formContent.trim()

    if (!cleanShortcut) {
      setError(t('Shortcut is required.'))
      return
    }
    if (!cleanTitle) {
      setError(t('Title is required.'))
      return
    }
    if (!cleanContent) {
      setError(t('Content is required.'))
      return
    }

    setSaving(true)
    setError('')
    try {
      if (editingItem) {
        await chatApi.updateCannedResponse(editingItem.id, {
          shortcut: cleanShortcut,
          title: cleanTitle,
          category: formCategory,
          content: cleanContent,
        })
      } else {
        await chatApi.createCannedResponse({
          shortcut: cleanShortcut,
          title: cleanTitle,
          category: formCategory,
          content: cleanContent,
        })
      }
      setIsCreating(false)
      setEditingItem(null)
      await loadResponses()
    } catch (err: any) {
      setError(err?.message || t('Could not save template.'))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm(t('Are you sure you want to delete this template?'))) return
    setLoading(true)
    try {
      await chatApi.deleteCannedResponse(id)
      await loadResponses()
    } catch (err: any) {
      setError(err?.message || t('Could not delete template.'))
    } finally {
      setLoading(false)
    }
  }

  const categories = ['all', 'general', 'greeting', 'sales', 'support', 'closing']

  return (
    <ActionDialog
      open={open}
      title={t('Canned responses')}
      onClose={onClose}
    >
      <div className="space-y-4 text-foreground">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {isCreating || editingItem ? (
          /* Create / Edit Form */
          <form onSubmit={handleSave} className="space-y-3.5">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <h3 className="font-semibold text-sm">
                {editingItem ? t('Edit template') : t('New template')}
              </h3>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  setIsCreating(false)
                  setEditingItem(null)
                }}
              >
                {t('Cancel')}
              </Button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1">
                  {t('Shortcut')} (e.g. halo)
                </label>
                <div className="flex items-center rounded-md border border-input bg-background px-2.5">
                  <span className="text-muted-foreground text-sm font-mono">/</span>
                  <input
                    type="text"
                    required
                    maxLength={50}
                    value={formShortcut}
                    onChange={(e) => setFormShortcut(e.target.value.replace(/\s+/g, ''))}
                    placeholder="shortcut"
                    className="w-full bg-transparent p-1.5 text-sm outline-none font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1">
                  {t('Category')}
                </label>
                <select
                  value={formCategory}
                  onChange={(e) => setFormCategory(e.target.value)}
                  className="w-full rounded-md border border-input bg-background p-2 text-sm outline-none"
                >
                  <option value="general">{t('General')}</option>
                  <option value="greeting">{t('Greeting')}</option>
                  <option value="sales">{t('Sales')}</option>
                  <option value="support">{t('Support')}</option>
                  <option value="closing">{t('Closing')}</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                {t('Title')}
              </label>
              <Input
                type="text"
                required
                maxLength={120}
                value={formTitle}
                onChange={(e) => setFormTitle(e.target.value)}
                placeholder={t('e.g. Salam Pembuka')}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                {t('Content')}
              </label>
              <textarea
                required
                rows={4}
                maxLength={6000}
                value={formContent}
                onChange={(e) => setFormContent(e.target.value)}
                placeholder={t('Write the reply template text…')}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm outline-none resize-y min-h-[90px]"
              />
            </div>

            <div className="flex justify-end gap-2 pt-2 border-t border-border">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setIsCreating(false)
                  setEditingItem(null)
                }}
              >
                {t('Cancel')}
              </Button>
              <Button type="submit" size="sm" disabled={saving}>
                {saving ? t('Saving…') : t('Save template')}
              </Button>
            </div>
          </form>
        ) : (
          /* List View */
          <>
            <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center justify-between">
              <div className="relative flex-1">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  type="text"
                  placeholder={t('Search templates…')}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="pl-9 h-9 text-sm"
                />
              </div>
              <Button size="sm" onClick={handleStartCreate} className="gap-1.5 h-9 shrink-0">
                <Plus className="h-4 w-4" />
                <span>{t('New template')}</span>
              </Button>
            </div>

            {/* Category Filter Pills */}
            <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
              {categories.map((cat) => (
                <button
                  key={cat}
                  type="button"
                  onClick={() => setCategory(cat)}
                  className={`px-2.5 py-1 rounded-full capitalize font-medium transition-colors shrink-0 ${
                    category === cat
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted/60 text-muted-foreground hover:bg-muted'
                  }`}
                >
                  {cat === 'all' ? t('All') : cat}
                </button>
              ))}
            </div>

            {/* Template List */}
            <div className="max-h-[320px] overflow-y-auto space-y-2 pr-1">
              {loading && items.length === 0 ? (
                <p className="text-center text-xs text-muted-foreground py-6">
                  {t('Loading templates…')}
                </p>
              ) : items.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground space-y-2">
                  <p className="text-sm font-medium">{t('No canned responses found')}</p>
                  <p className="text-xs">{t('Add saved reply templates to speed up customer replies.')}</p>
                </div>
              ) : (
                items.map((item) => (
                  <div
                    key={item.id}
                    className="p-3 rounded-lg border border-border bg-card hover:border-primary/40 transition-all flex flex-col gap-1.5 group"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 min-w-0">
                        <span className="font-mono text-xs font-semibold px-2 py-0.5 rounded bg-primary/10 text-primary border border-primary/20 shrink-0">
                          /{item.shortcut}
                        </span>
                        <span className="font-semibold text-sm truncate">{item.title}</span>
                        {item.category && item.category !== 'general' && (
                          <span className="text-[10px] uppercase font-bold tracking-wider px-1.5 py-0.5 rounded bg-muted text-muted-foreground shrink-0">
                            {item.category}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        {onSelect && (
                          <Button
                            type="button"
                            size="sm"
                            variant="secondary"
                            onClick={() => {
                              onSelect(item.content)
                              onClose()
                            }}
                            className="h-7 px-2 text-xs gap-1"
                          >
                            <Check className="h-3 w-3" />
                            <span>{t('Use')}</span>
                          </Button>
                        )}
                        <button
                          type="button"
                          onClick={() => handleStartEdit(item)}
                          className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted"
                          title={t('Edit template')}
                        >
                          <Edit2 className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDelete(item.id)}
                          className="p-1 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                          title={t('Delete template')}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                    <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed whitespace-pre-wrap">
                      {item.content}
                    </p>
                  </div>
                ))
              )}
            </div>
          </>
        )}
      </div>
    </ActionDialog>
  )
}
