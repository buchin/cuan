import { useState, useEffect, useRef, useCallback } from 'react'
import { Zap, Search, Settings2 } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { chatApi, CannedResponse } from '../../api/chat'

export const CANNED_RESPONSES_CHANGED_EVENT = 'cuan-canned-responses-changed'

interface CannedResponsesPickerProps {
  onSelect: (content: string) => void
  onOpenManage: () => void
  disabled?: boolean
  inputMessage: string
}

export function CannedResponsesPicker({
  onSelect,
  onOpenManage,
  disabled = false,
  inputMessage,
}: CannedResponsesPickerProps) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const [items, setItems] = useState<CannedResponse[]>([])
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const popoverRef = useRef<HTMLDivElement>(null)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  // Fetch canned responses from server (coalesced via apiFetch GET deduplication)
  const fetchResponses = useCallback(async () => {
    setLoading(true)
    try {
      const data = await chatApi.listCannedResponses()
      if (mountedRef.current) {
        setItems(data)
      }
    } catch {
      // ignore
    } finally {
      if (mountedRef.current) {
        setLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    if (isOpen) {
      void fetchResponses()
    }
  }, [isOpen, fetchResponses])

  // Close when clicking outside or pressing Escape
  useEffect(() => {
    if (!isOpen) return
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

  // Autocomplete detection: check if input starts with `/` or ends with `/shortcut`
  const slashMatch = inputMessage.match(/(?:^|\s)\/([a-zA-Z0-9_-]*)$/)
  const isSlashActive = !isOpen && !!slashMatch
  const slashQuery = slashMatch ? slashMatch[1].toLowerCase() : ''

  // Load items on mount and keep synced across devices/tabs on focus, visibility, and local mutations
  useEffect(() => {
    void fetchResponses()
    const handleRefresh = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
      void fetchResponses()
    }
    window.addEventListener('focus', handleRefresh)
    document.addEventListener('visibilitychange', handleRefresh)
    window.addEventListener(CANNED_RESPONSES_CHANGED_EVENT, handleRefresh)
    return () => {
      window.removeEventListener('focus', handleRefresh)
      document.removeEventListener('visibilitychange', handleRefresh)
      window.removeEventListener(CANNED_RESPONSES_CHANGED_EVENT, handleRefresh)
    }
  }, [fetchResponses])

  // Refetch when slash autocomplete activates so templates added on another device appear immediately
  useEffect(() => {
    if (isSlashActive) {
      void fetchResponses()
    }
  }, [isSlashActive, fetchResponses])

  const filteredItems = items.filter((item) => {
    const q = (isOpen ? search : slashQuery).toLowerCase()
    if (!q) return true
    return (
      item.shortcut.toLowerCase().includes(q) ||
      item.title.toLowerCase().includes(q) ||
      item.content.toLowerCase().includes(q)
    )
  })

  const handleApply = (content: string) => {
    onSelect(content)
    setIsOpen(false)
    setSearch('')
  }

  return (
    <div className="sm:relative inline-block" ref={popoverRef}>
      {/* Trigger Button inside composer */}
      <button
        type="button"
        disabled={disabled}
        onClick={() => setIsOpen((prev) => !prev)}
        className="wa-action-btn flex h-9 w-9 sm:h-9.5 sm:w-9.5 items-center justify-center rounded-full active:scale-95 disabled:pointer-events-none disabled:opacity-40 shrink-0 cursor-pointer"
        title={t('Canned responses') + ' (/)'}
        aria-label={t('Canned responses')}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
      >
        <Zap className="h-[18px] w-[18px]" />
      </button>

      {/* 1. Modal / Floating Picker when clicking the Zap button */}
      {isOpen && (
        <div
          role="dialog"
          aria-label={t('Saved replies')}
          className="absolute left-0 right-0 bottom-full mb-2 z-50 sm:left-auto sm:right-0 sm:bottom-12 sm:mb-0 sm:w-80 rounded-2xl border border-border/60 bg-card/95 backdrop-blur-md p-3 text-card-foreground shadow-xl animate-in fade-in slide-in-from-bottom-2 duration-150"
        >
          <div className="flex items-center justify-between px-0.5 pb-2.5 border-b border-border/40 mb-2.5">
            <span className="font-semibold text-xs flex items-center gap-1.5 text-foreground">
              <Zap className="h-3.5 w-3.5 text-primary" />
              <span>{t('Saved replies')}</span>
            </span>
            <button
              type="button"
              onClick={() => {
                setIsOpen(false)
                onOpenManage()
              }}
              className="text-[11px] font-medium text-muted-foreground hover:text-foreground flex items-center gap-1 px-1.5 py-1 rounded-md hover:bg-muted/70 transition-colors cursor-pointer"
              title={t('Manage templates')}
            >
              <Settings2 className="h-3 w-3" />
              <span>{t('Manage')}</span>
            </button>
          </div>

          <div className="relative mb-2">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/70" />
            <input
              type="text"
              autoFocus
              placeholder={t('Search templates…')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-lg border border-transparent bg-muted/55 pl-8 pr-3 py-2 text-xs text-foreground placeholder:text-muted-foreground/70 outline-none transition-colors focus:bg-muted/80 focus:border-border/50"
            />
          </div>

          <div className="max-h-64 sm:max-h-60 overflow-y-auto space-y-0.5 pr-0.5">
            {loading && items.length === 0 ? (
              <p className="py-5 text-center text-xs text-muted-foreground">
                {t('Loading…')}
              </p>
            ) : filteredItems.length === 0 ? (
              <div className="py-5 text-center text-xs text-muted-foreground">
                <p>{t('No templates found')}</p>
                <button
                  type="button"
                  onClick={() => {
                    setIsOpen(false)
                    onOpenManage()
                  }}
                  className="mt-1.5 text-primary hover:underline text-xs cursor-pointer"
                >
                  {t('+ Add template')}
                </button>
              </div>
            ) : (
              filteredItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => handleApply(item.content)}
                  className="w-full text-left px-2.5 py-2.5 sm:py-2 rounded-lg hover:bg-muted/60 active:bg-muted transition-colors flex flex-col gap-1 group cursor-pointer"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[13px] sm:text-xs font-medium text-foreground truncate flex-1">
                      {item.title}
                    </span>
                    <span className="font-mono text-[11px] font-medium text-muted-foreground/80 shrink-0">
                      /{item.shortcut}
                    </span>
                  </div>
                  <p className="text-xs sm:text-[11px] text-muted-foreground/85 line-clamp-1 leading-relaxed">
                    {item.content}
                  </p>
                </button>
              ))
            )}
          </div>
        </div>
      )}

      {/* 2. Slash Autocomplete Popup when user types `/` in textarea */}
      {isSlashActive && filteredItems.length > 0 && (
        <div className="absolute left-0 right-0 bottom-full mb-2 z-50 sm:left-auto sm:right-0 sm:bottom-12 sm:mb-0 sm:w-80 rounded-2xl border border-border/60 bg-card/95 backdrop-blur-md p-2.5 text-card-foreground shadow-xl animate-in fade-in slide-in-from-bottom-2 duration-150">
          <div className="px-1.5 pb-2 border-b border-border/40 mb-1.5 flex items-center justify-between text-[11px] text-muted-foreground">
            <span className="font-medium">{t('Matching templates')}</span>
            <span className="font-mono text-[10px]">/{slashQuery}</span>
          </div>
          <div className="max-h-56 overflow-y-auto space-y-0.5">
            {filteredItems.slice(0, 5).map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => {
                  // Replace slash shortcut in input
                  const beforeSlash = inputMessage.replace(/(?:^|\s)\/[a-zA-Z0-9_-]*$/, '')
                  const nextText = beforeSlash ? `${beforeSlash.trimEnd()} ${item.content}` : item.content
                  handleApply(nextText)
                }}
                className="w-full text-left px-2.5 py-2.5 sm:py-2 rounded-lg hover:bg-muted/60 active:bg-muted transition-colors flex flex-col gap-1 cursor-pointer"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[13px] sm:text-xs font-medium text-foreground truncate flex-1">
                    {item.title}
                  </span>
                  <span className="font-mono text-[11px] font-medium text-muted-foreground/80 shrink-0">
                    /{item.shortcut}
                  </span>
                </div>
                <p className="text-xs sm:text-[11px] text-muted-foreground/85 line-clamp-1 leading-relaxed">
                  {item.content}
                </p>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
