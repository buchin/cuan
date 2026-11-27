import { useEffect, useRef, useState } from 'react'
import { Edit2, Plus, Search, Settings2, Trash2, Zap } from 'lucide-react'
import { ActionDialog } from '../../../components/dialogs/ActionDialog'
import { Button } from '../../../components/ui/button'
import { FormField, FormGrid, Input, Textarea } from '../../../components/ui/forms'
import { useLanguage } from '../../../context/LanguageContext'
import {
  filterTerminalQuickReplies,
  loadTerminalQuickReplies,
  saveTerminalQuickReplies,
  type TerminalQuickReply,
} from './terminalQuickReplies'

export interface TerminalQuickRepliesPickerProps {
  onSelect: (content: string) => void
  disabled?: boolean
}

export function TerminalQuickRepliesPicker({
  onSelect,
  disabled = false,
}: TerminalQuickRepliesPickerProps) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const [manageOpen, setManageOpen] = useState(false)
  const [items, setItems] = useState<TerminalQuickReply[]>(() => loadTerminalQuickReplies())
  const [search, setSearch] = useState('')
  const [manageSearch, setManageSearch] = useState('')
  const [editingItem, setEditingItem] = useState<TerminalQuickReply | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [formTitle, setFormTitle] = useState('')
  const [formShortcut, setFormShortcut] = useState('')
  const [formContent, setFormContent] = useState('')
  const [formError, setFormError] = useState('')
  const popoverRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!isOpen) return
    setItems(loadTerminalQuickReplies())
    const handleClickOutside = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEscape)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [isOpen])

  const filteredItems = filterTerminalQuickReplies(items, search)
  const filteredManageItems = filterTerminalQuickReplies(items, manageSearch)

  const handleApply = (content: string) => {
    onSelect(content)
    setIsOpen(false)
    setSearch('')
  }

  const openManageDialog = () => {
    setIsOpen(false)
    setItems(loadTerminalQuickReplies())
    setIsCreating(false)
    setEditingItem(null)
    setFormError('')
    setManageOpen(true)
  }

  const handleStartCreate = () => {
    setFormTitle('')
    setFormShortcut('')
    setFormContent('')
    setFormError('')
    setEditingItem(null)
    setIsCreating(true)
  }

  const handleStartEdit = (item: TerminalQuickReply) => {
    setFormTitle(item.title)
    setFormShortcut(item.shortcut || '')
    setFormContent(item.content)
    setFormError('')
    setIsCreating(false)
    setEditingItem(item)
  }

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault()
    const cleanTitle = formTitle.trim()
    const cleanShortcut = formShortcut.trim().replace(/^\/+/, '')
    const cleanContent = formContent.trim()

    if (!cleanTitle) {
      setFormError(t('Title is required.'))
      return
    }
    if (!cleanContent) {
      setFormError(t('Content is required.'))
      return
    }

    let next: TerminalQuickReply[]
    if (editingItem) {
      next = items.map((it) =>
        it.id === editingItem.id
          ? {
              ...it,
              title: cleanTitle,
              content: cleanContent,
              ...(cleanShortcut ? { shortcut: cleanShortcut } : { shortcut: undefined }),
            }
          : it,
      )
    } else {
      const id =
        typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
          ? `term-qr-${crypto.randomUUID()}`
          : `term-qr-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
      next = [
        {
          id,
          title: cleanTitle,
          content: cleanContent,
          ...(cleanShortcut ? { shortcut: cleanShortcut } : {}),
        },
        ...items,
      ]
    }

    saveTerminalQuickReplies(next)
    setItems(next)
    setIsCreating(false)
    setEditingItem(null)
    setFormError('')
  }

  const handleDelete = (id: string) => {
    const next = items.filter((it) => it.id !== id)
    saveTerminalQuickReplies(next)
    setItems(next)
  }

  return (
    <div className="relative inline-block" ref={popoverRef}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setIsOpen((prev) => !prev)}
        className="wa-action-btn flex h-9 w-9 sm:h-9.5 sm:w-9.5 items-center justify-center rounded-full active:scale-95 disabled:pointer-events-none disabled:opacity-40 shrink-0 cursor-pointer"
        title={t('Quick replies')}
        aria-label={t('Quick replies')}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        data-testid="terminal-quick-reply-trigger"
      >
        <Zap className="h-[18px] w-[18px]" />
      </button>

      {isOpen && (
        <div
          role="dialog"
          aria-label={t('Quick replies')}
          className="absolute bottom-12 right-0 z-50 w-72 sm:w-80 max-w-[calc(100vw-24px)] rounded-xl border border-border bg-card p-2.5 text-card-foreground shadow-2xl animate-in fade-in slide-in-from-bottom-2 duration-150"
          data-testid="terminal-quick-reply-popover"
        >
          <div className="flex items-center justify-between pb-2 border-b border-border/70 mb-2">
            <span className="font-semibold text-xs flex items-center gap-1.5 text-foreground">
              <Zap className="h-3.5 w-3.5 text-primary" />
              <span>{t('Quick replies')}</span>
            </span>
            <button
              type="button"
              onClick={openManageDialog}
              className="text-[11px] font-medium text-muted-foreground hover:text-foreground flex items-center gap-1 p-1 rounded hover:bg-muted transition-colors cursor-pointer"
              title={t('Manage templates')}
              data-testid="terminal-quick-reply-manage"
            >
              <Settings2 className="h-3 w-3" />
              <span>{t('Manage')}</span>
            </button>
          </div>

          <div className="relative mb-2">
            <Search className="absolute left-2 top-2 h-3.5 w-3.5 text-muted-foreground" />
            <input
              type="text"
              autoFocus
              placeholder={t('Search templates')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-md border border-input bg-background pl-7 pr-2 py-1 text-xs text-foreground outline-none focus:border-primary"
              data-testid="terminal-quick-reply-search"
            />
          </div>

          <div className="max-h-56 overflow-y-auto space-y-1 pr-0.5">
            {filteredItems.length === 0 ? (
              <div className="py-4 text-center text-xs text-muted-foreground">
                <p>{t('No templates found')}</p>
                <button
                  type="button"
                  onClick={() => {
                    openManageDialog()
                    handleStartCreate()
                  }}
                  className="mt-1 text-primary hover:underline text-xs cursor-pointer"
                >
                  + {t('New template')}
                </button>
              </div>
            ) : (
              filteredItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => handleApply(item.content)}
                  className="w-full text-left p-2 rounded-lg hover:bg-muted/70 active:bg-muted transition-colors flex flex-col gap-0.5 group border border-transparent hover:border-border/60 cursor-pointer"
                  data-testid="terminal-quick-reply-item"
                >
                  <div className="flex items-center justify-between gap-1.5">
                    <span className="text-xs font-semibold text-foreground truncate flex-1">
                      {item.title}
                    </span>
                    {item.shortcut && (
                      <span className="font-mono text-[10px] font-medium text-muted-foreground shrink-0">
                        {item.shortcut}
                      </span>
                    )}
                  </div>
                  <p className="text-[11px] text-muted-foreground line-clamp-1 leading-snug">
                    {item.content}
                  </p>
                </button>
              ))
            )}
          </div>
        </div>
      )}

      <ActionDialog
        open={manageOpen}
        title={t('Quick replies')}
        onClose={() => {
          setManageOpen(false)
          setIsCreating(false)
          setEditingItem(null)
        }}
      >
        <div className="space-y-4 text-foreground">
          {formError && (
            <p className="rounded-lg bg-destructive/15 px-3 py-2 text-xs text-destructive border border-destructive/20">
              {formError}
            </p>
          )}

          {isCreating || editingItem ? (
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
                    setFormError('')
                  }}
                >
                  {t('Cancel')}
                </Button>
              </div>

              <FormGrid cols={2}>
                <FormField label={t('Title')}>
                  <Input
                    type="text"
                    required
                    maxLength={80}
                    value={formTitle}
                    onChange={(e) => setFormTitle(e.target.value)}
                    placeholder="e.g. Jalankan test"
                  />
                </FormField>

                <FormField label={`${t('Shortcut')} (opsional)`}>
                  <Input
                    type="text"
                    maxLength={32}
                    mono
                    value={formShortcut}
                    onChange={(e) => setFormShortcut(e.target.value.replace(/\s+/g, ''))}
                    placeholder="test"
                  />
                </FormField>
              </FormGrid>

              <FormField label={t('Content')}>
                <Textarea
                  required
                  rows={3}
                  maxLength={4000}
                  value={formContent}
                  onChange={(e) => setFormContent(e.target.value)}
                  placeholder="Tulis perintah atau prompt yang sering dipakai…"
                  className="min-h-[84px]"
                />
              </FormField>

              <div className="flex justify-end gap-2 pt-2 border-t border-border">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setIsCreating(false)
                    setEditingItem(null)
                    setFormError('')
                  }}
                >
                  {t('Cancel')}
                </Button>
                <Button type="submit" size="sm">
                  {t('Save')}
                </Button>
              </div>
            </form>
          ) : (
            <>
              <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center justify-between">
                <div className="relative flex-1">
                  <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    type="text"
                    placeholder={t('Search templates')}
                    value={manageSearch}
                    onChange={(e) => setManageSearch(e.target.value)}
                    className="pl-9 h-9 text-sm"
                  />
                </div>
                <Button size="sm" onClick={handleStartCreate} className="gap-1.5 h-9 shrink-0">
                  <Plus className="h-4 w-4" />
                  <span>{t('New template')}</span>
                </Button>
              </div>

              <div className="max-h-72 overflow-y-auto space-y-1.5 pr-0.5">
                {filteredManageItems.length === 0 ? (
                  <p className="py-6 text-center text-xs text-muted-foreground">
                    {t('No templates found')}
                  </p>
                ) : (
                  filteredManageItems.map((item) => (
                    <div
                      key={item.id}
                      className="flex items-start justify-between gap-2 rounded-lg border border-border/70 bg-muted/20 p-2.5"
                    >
                      <button
                        type="button"
                        onClick={() => {
                          onSelect(item.content)
                          setManageOpen(false)
                        }}
                        className="min-w-0 flex-1 text-left cursor-pointer"
                      >
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-semibold text-foreground truncate">
                            {item.title}
                          </span>
                          {item.shortcut && (
                            <span className="font-mono text-[10px] text-muted-foreground shrink-0">
                              {item.shortcut}
                            </span>
                          )}
                        </div>
                        <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
                          {item.content}
                        </p>
                      </button>
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          type="button"
                          onClick={() => handleStartEdit(item)}
                          className="p-1.5 rounded text-muted-foreground hover:text-foreground hover:bg-muted transition-colors cursor-pointer"
                          title={t('Edit template')}
                          aria-label={t('Edit template')}
                        >
                          <Edit2 className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDelete(item.id)}
                          className="p-1.5 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
                          title={t('Delete')}
                          aria-label={t('Delete')}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </>
          )}
        </div>
      </ActionDialog>
    </div>
  )
}
