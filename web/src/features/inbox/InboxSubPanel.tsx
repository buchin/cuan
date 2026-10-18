import { InboxThreadItem } from '../../api/inbox'
import { Plus, MessageSquare, CheckCircle2, Clock, AlertCircle } from 'lucide-react'
import { cn } from '../../lib/utils'

interface InboxSubPanelProps {
  threads: InboxThreadItem[]
  activeThreadId?: string
  onSelectThread: (thread: InboxThreadItem) => void
  onCreateThreadClick: () => void
  statusFilter: string
  onStatusFilterChange: (status: string) => void
}

const FILTERS = [
  { id: 'all', label: 'All' },
  { id: 'unread', label: 'Unread' },
  { id: 'in_progress', label: 'In Progress' },
  { id: 'done', label: 'Done' },
]

export function InboxSubPanel({
  threads,
  activeThreadId,
  onSelectThread,
  onCreateThreadClick,
  statusFilter,
  onStatusFilterChange,
}: InboxSubPanelProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'done':
        return (
          <span className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="h-2.5 w-2.5" />
            Done
          </span>
        )
      case 'in_progress':
        return (
          <span className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">
            <Clock className="h-2.5 w-2.5" />
            Running
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
            <AlertCircle className="h-2.5 w-2.5" />
            Inbox
          </span>
        )
    }
  }

  const formatTime = (isoString: string) => {
    try {
      const d = new Date(isoString)
      const now = new Date()
      const diffMs = now.getTime() - d.getTime()
      const diffMins = Math.floor(diffMs / 60000)
      if (diffMins < 1) return 'now'
      if (diffMins < 60) return `${diffMins}m`
      const diffHours = Math.floor(diffMins / 60)
      if (diffHours < 24) return `${diffHours}h`
      return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
    } catch {
      return ''
    }
  }

  return (
    <div className="flex flex-col h-full bg-sidebar/50">
      {/* Top Action: New Thread */}
      <div className="p-3 border-b border-border">
        <button
          onClick={onCreateThreadClick}
          className="flex items-center justify-center gap-2 w-full px-3 py-2 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>New Thread</span>
        </button>
      </div>

      {/* Filter Tabs */}
      <div className="px-2 py-2 border-b border-border flex items-center gap-1 overflow-x-auto text-[11px]">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            onClick={() => onStatusFilterChange(f.id)}
            className={cn(
              'px-2 py-1 rounded transition-colors whitespace-nowrap',
              statusFilter === f.id
                ? 'bg-accent text-accent-foreground font-semibold'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {/* Thread List */}
      <div className="flex-1 overflow-y-auto divide-y divide-border/50">
        {threads.length === 0 ? (
          <div className="p-4 text-center text-muted-foreground text-xs italic">
            Tidak ada thread percakapan.
          </div>
        ) : (
          threads.map((thread) => {
            const isSelected = activeThreadId === thread.id
            const snippet = thread.last_message?.content || thread.topic || 'Belum ada pesan'

            return (
              <button
                key={thread.id}
                onClick={() => onSelectThread(thread)}
                className={cn(
                  'w-full p-3 text-left transition-colors flex flex-col gap-1.5',
                  isSelected
                    ? 'bg-accent/80 border-l-2 border-l-primary'
                    : 'hover:bg-accent/40'
                )}
              >
                <div className="flex items-start justify-between gap-1 w-full">
                  <span className="font-semibold text-xs text-foreground truncate max-w-[150px]">
                    {thread.title}
                  </span>
                  <span className="text-[10px] text-muted-foreground shrink-0 font-mono">
                    {formatTime(thread.updated_at || thread.created_at)}
                  </span>
                </div>

                <p className="text-[11px] text-muted-foreground line-clamp-2 leading-relaxed">
                  {snippet}
                </p>

                <div className="flex items-center justify-between gap-1 mt-0.5 pt-1 border-t border-border/30 text-[10px]">
                  <span className="text-muted-foreground truncate max-w-[100px] font-mono">
                    {thread.project_name}
                  </span>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <span className="text-muted-foreground flex items-center gap-0.5 font-mono">
                      <MessageSquare className="h-2.5 w-2.5" />
                      {thread.message_count}
                    </span>
                    {getStatusBadge(thread.status)}
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
