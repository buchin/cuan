import { useState, memo } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { ChatMarkdown } from './ChatMarkdown'
import { cn } from '../../lib/utils'
import type { Task } from '../../api/types'

export interface CollapsibleChatTextProps {
  content: string
  maxLines?: number
  maxChars?: number
  hideTaskCard?: boolean
  onTaskClick?: (task: Task) => void
  isCustomerRoom?: boolean
  className?: string
  isOutgoing?: boolean
}

export function CollapsibleChatText(props: CollapsibleChatTextProps) {
  return <MemoizedCollapsibleChatText {...props} />
}

const MemoizedCollapsibleChatText = memo(function MemoizedCollapsibleChatText({
  content,
  maxLines = 7,
  maxChars = 420,
  hideTaskCard = false,
  onTaskClick,
  isCustomerRoom,
  className,
  isOutgoing = false,
}: CollapsibleChatTextProps) {
  const { t } = useLanguage()
  const [expanded, setExpanded] = useState(false)

  const lineCount = content.split('\n').filter(Boolean).length
  const isLong = content.length > maxChars || lineCount > maxLines

  if (!isLong) {
    return (
      <div className={cn('min-w-0 max-w-full', className)}>
        <ChatMarkdown
          content={content}
          hideTaskCard={hideTaskCard}
          onTaskClick={onTaskClick}
          isCustomerRoom={isCustomerRoom}
        />
      </div>
    )
  }

  return (
    <div className={cn('min-w-0 max-w-full', className)}>
      <div
        className={cn(
          'relative transition-all duration-200',
          !expanded && 'max-h-[175px] overflow-hidden',
        )}
      >
        <ChatMarkdown
          content={content}
          hideTaskCard={hideTaskCard}
          onTaskClick={onTaskClick}
          isCustomerRoom={isCustomerRoom}
        />
        {!expanded && (
          <div
            className={cn(
              'absolute inset-x-0 bottom-0 h-14 bg-gradient-to-t pointer-events-none',
              isOutgoing
                ? 'from-[#25272c] via-[#25272c]/80 to-transparent'
                : 'from-[#1b1c1f] via-[#1b1c1f]/80 to-transparent',
            )}
          />
        )}
      </div>
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline cursor-pointer"
      >
        <span>{expanded ? t('Show less') : t('Read more')}</span>
        {expanded ? (
          <ChevronUp className="h-3 w-3" />
        ) : (
          <ChevronDown className="h-3 w-3" />
        )}
      </button>
    </div>
  )
})
