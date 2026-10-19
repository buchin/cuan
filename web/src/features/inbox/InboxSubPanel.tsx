import type { Agent } from '../../api/agents'
import { ContactStatus } from '../agents/ContactStatus'
import { useState } from 'react'
import { ChatContact } from '../../api/chat'
import { Search, Bot, User } from 'lucide-react'
import { cn } from '../../lib/utils'

export type ContactFilterTab = 'all' | 'agents' | 'customers'

interface InboxSubPanelProps {
  contacts: ChatContact[]
  agents?: Agent[]
  now?: number
  loading?: boolean
  error?: string
  onRetry?: () => void
  activeContactId?: string
  onSelectContact: (contact: ChatContact) => void
  selectedTab?: ContactFilterTab
  onTabChange?: (tab: ContactFilterTab) => void
  searchQuery?: string
  onSearchChange?: (q: string) => void
}

export function InboxSubPanel({
  contacts,
  agents = [],
  now,
  loading = false,
  error = '',
  onRetry,
  activeContactId,
  onSelectContact,
  selectedTab = 'all',
  onTabChange,
  searchQuery = '',
  onSearchChange,
}: InboxSubPanelProps) {
  const [internalTab, setInternalTab] = useState<ContactFilterTab>(selectedTab)
  const [internalQuery, setInternalQuery] = useState(searchQuery)

  const activeTab = onTabChange ? selectedTab : internalTab
  const handleTabClick = (tab: ContactFilterTab) => {
    if (onTabChange) onTabChange(tab)
    else setInternalTab(tab)
  }

  const query = onSearchChange ? searchQuery : internalQuery
  const handleQueryChange = (val: string) => {
    if (onSearchChange) onSearchChange(val)
    else setInternalQuery(val)
  }

  const agentCount = contacts.filter((c) => c.type === 'agent').length
  const customerCount = contacts.filter((c) => c.type === 'customer').length

  const filteredContacts = contacts.filter((c) => {
    if (activeTab === 'agents' && c.type !== 'agent') return false
    if (activeTab === 'customers' && c.type !== 'customer') return false
    if (query.trim()) {
      const q = query.toLowerCase()
      const matchName = c.name.toLowerCase().includes(q)
      const matchMsg = (c.last_message || '').toLowerCase().includes(q)
      const matchProduct = (c.product || '').toLowerCase().includes(q)
      return matchName || matchMsg || matchProduct
    }
    return true
  })

  const formatTime = (iso?: string) => {
    if (!iso) return ''
    try {
      const d = new Date(iso)
      const now = new Date()
      const isToday = d.toDateString() === now.toDateString()
      if (isToday) {
        return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      }
      return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
    } catch {
      return ''
    }
  }

  return (
    <div className="flex flex-col h-full bg-sidebar/50 text-sidebar-foreground select-none">
      {/* Sub-panel Top: Header & Search */}
      <div className="p-3 border-b border-border space-y-2.5">
        <div className="flex items-center justify-between">
          <span className="text-xs font-bold tracking-tight text-foreground uppercase">
            Percakapan
          </span>
          <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
            {contacts.length} percakapan
          </span>
        </div>

        {/* Search Bar */}
        <div className="relative">
          <Search className="absolute left-2.5 top-3.5 h-3.5 w-3.5 text-muted-foreground" />
          <input
            type="text"
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            aria-label="Cari percakapan"
            placeholder="Cari percakapan…"
            className="w-full min-h-11 pl-8 pr-3 py-2 text-base rounded-md bg-background border border-input text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>

        {/* Filter Tabs (All, Agents, Customers) */}
        <div className="flex rounded-md bg-muted p-0.5 text-xs">
          <button
            onClick={() => handleTabClick('all')}
            className={cn(
              'flex-1 min-h-11 py-2 rounded text-center transition-all font-medium text-sm',
              activeTab === 'all'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            Semua
          </button>
          <button
            onClick={() => handleTabClick('agents')}
            className={cn(
              'flex-1 min-h-11 py-2 rounded text-center transition-all font-medium text-sm flex items-center justify-center gap-1',
              activeTab === 'agents'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            <span>Agen</span>
            <span className="sr-only">({agentCount})</span>
          </button>
          <button
            onClick={() => handleTabClick('customers')}
            className={cn(
              'flex-1 min-h-11 py-2 rounded text-center transition-all font-medium text-sm flex items-center justify-center gap-1',
              activeTab === 'customers'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            <span>Pelanggan</span>
            <span className="sr-only">({customerCount})</span>
          </button>
        </div>
      </div>

      {/* Contacts List */}
      <div className="flex-1 overflow-y-auto divide-y divide-border/40">
        {loading ? (
          <p role="status" className="p-4 text-sm">
            Memuat percakapan…
          </p>
        ) : error ? (
          <div role="alert" className="p-4 text-sm">
            <p>{error}</p>
            <button className="action-secondary mt-3" onClick={onRetry}>
              Coba lagi
            </button>
          </div>
        ) : filteredContacts.length === 0 ? (
          <div className="p-6 text-center text-xs text-muted-foreground">
            Belum ada percakapan yang cocok.
          </div>
        ) : (
          filteredContacts.map((contact) => {
            const isSelected =
              activeContactId === contact.id ||
              activeContactId === contact.conversation_id
            return (
              <button
                key={contact.id}
                onClick={() => onSelectContact(contact)}
                className={cn(
                  'w-full text-left p-3 transition-colors flex items-start gap-3 hover:bg-accent/40',
                  isSelected && 'bg-accent text-accent-foreground font-medium',
                )}
              >
                {/* Avatar with Online Dot */}
                <div className="relative shrink-0 mt-0.5">
                  <div className="h-9 w-9 rounded-full bg-secondary flex items-center justify-center text-sm font-semibold border border-border shadow-xs">
                    {contact.avatar ? (
                      <span>{contact.avatar}</span>
                    ) : contact.type === 'agent' ? (
                      <Bot className="h-4 w-4 text-primary" />
                    ) : (
                      <User className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                </div>

                {/* Details */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-1 mb-0.5">
                    <span className="text-sm font-semibold text-foreground truncate">
                      {contact.name}
                    </span>
                    <span className="text-xs text-muted-foreground shrink-0 font-mono">
                      {formatTime(
                        contact.last_message_at || contact.updated_at,
                      )}
                    </span>
                  </div>

                  {/* Badge & Last Message */}
                  <div className="flex items-center gap-1.5 mb-1">
                    <ContactStatus
                      contact={contact}
                      agents={agents}
                      now={now}
                    />
                  </div>

                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm text-muted-foreground truncate leading-relaxed">
                      {contact.last_message || 'Belum ada pesan.'}
                    </p>
                    {contact.unread_count > 0 && (
                      <span className="shrink-0 h-4 min-w-4 px-1 rounded-full bg-primary text-primary-foreground text-xs font-bold flex items-center justify-center">
                        {contact.unread_count}
                      </span>
                    )}
                  </div>
                </div>
              </button>
            )
          })
        )}
      </div>
    </div>
  )
}
