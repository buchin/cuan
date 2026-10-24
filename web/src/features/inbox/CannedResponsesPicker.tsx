import { useState, useEffect, useRef } from 'react'
import { Zap, Search, Settings2, Check } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { chatApi, CannedResponse } from '../../api/chat'

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

  // Fetch canned responses
  const fetchResponses = async () => {
    setLoading(true)
    try {
      const data = await chatApi.listCannedResponses()
      setItems(data)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      fetchResponses()
    }
  }, [isOpen])

  // Close when clicking outside
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen])

  // Autocomplete detection: check if input starts with `/` or ends with `/shortcut`
  const slashMatch = inputMessage.match(/(?:^|\s)\/([a-zA-Z0-9_-]*)$/)
  const isSlashActive = !isOpen && !!slashMatch
  const slashQuery = slashMatch ? slashMatch[1].toLowerCase() : ''

  // Load items on mount for slash autocomplete
  useEffect(() => {
    chatApi.listCannedResponses().then(setItems).catch(() => {})
  }, [])

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
    <div className="relative inline-block" ref={popoverRef}>
      {/* Trigger Button inside composer */}
      <button
        type="button"
        disabled={disabled}
        onClick={() => setIsOpen((prev) => !prev)}
        className="wa-attach-button flex h-11 w-11 items-center justify-center rounded-full text-muted-foreground transition-all hover:bg-muted hover:text-foreground active:scale-95 disabled:pointer-events-none disabled:opacity-40"
        title={t('Canned responses') + ' (/)'}
        aria-label={t('Canned responses')}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
      >
        <Zap className="h-4 w-4" />
      </button>

      {/* 1. Modal / Floating Picker when clicking the Zap button */}
      {isOpen && (
        <div className="absolute bottom-14 left-0 z-50 w-72 sm:w-80 rounded-xl border border-border bg-card p-2.5 text-card-foreground shadow-2xl animate-in fade-in slide-in-from-bottom-2 duration-150">
          <div className="flex items-center justify-between pb-2 border-b border-border/70 mb-2">
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
              className="text-[11px] font-medium text-muted-foreground hover:text-foreground flex items-center gap-1 p-1 rounded hover:bg-muted transition-colors"
              title={t('Manage templates')}
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
              placeholder={t('Search templates…')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-md border border-input bg-background pl-7 pr-2 py-1 text-xs outline-none focus:border-primary"
            />
          </div>

          <div className="max-h-56 overflow-y-auto space-y-1 pr-0.5">
            {loading && items.length === 0 ? (
              <p className="py-4 text-center text-xs text-muted-foreground">
                {t('Loading…')}
              </p>
            ) : filteredItems.length === 0 ? (
              <div className="py-4 text-center text-xs text-muted-foreground">
                <p>{t('No templates found')}</p>
                <button
                  type="button"
                  onClick={() => {
                    setIsOpen(false)
                    onOpenManage()
                  }}
                  className="mt-1 text-primary hover:underline text-xs"
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
                  className="w-full text-left p-2 rounded-lg hover:bg-muted/70 active:bg-muted transition-colors flex flex-col gap-0.5 group border border-transparent hover:border-border/60"
                >
                  <div className="flex items-center justify-between gap-1.5">
                    <span className="font-mono text-[11px] font-bold text-primary">
                      /{item.shortcut}
                    </span>
                    <span className="text-xs font-semibold text-foreground truncate flex-1 ml-1.5">
                      {item.title}
                    </span>
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

      {/* 2. Slash Autocomplete Popup when user types `/` in textarea */}
      {isSlashActive && filteredItems.length > 0 && (
        <div className="absolute bottom-14 left-0 z-50 w-72 sm:w-80 rounded-xl border border-border bg-card p-2 text-card-foreground shadow-2xl animate-in fade-in slide-in-from-bottom-2 duration-150">
          <div className="px-2 py-1 border-b border-border/60 mb-1 flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t('Matching templates')}</span>
            <span className="font-mono text-[10px]">/{slashQuery}</span>
          </div>
          <div className="max-h-48 overflow-y-auto space-y-1">
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
                className="w-full text-left p-2 rounded-md hover:bg-muted active:bg-muted/80 transition-colors flex flex-col gap-0.5"
              >
                <div className="flex items-center justify-between">
                  <span className="font-mono text-xs font-bold text-primary">
                    /{item.shortcut}
                  </span>
                  <span className="text-xs font-semibold text-foreground truncate ml-2">
                    {item.title}
                  </span>
                </div>
                <p className="text-[11px] text-muted-foreground line-clamp-1">
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
