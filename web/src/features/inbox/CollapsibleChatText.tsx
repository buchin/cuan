import { useState, useEffect, useRef, useId, memo } from 'react'
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
  markdownClassName?: string
  expandLabel?: string
  collapseLabel?: string
  isOutgoing?: boolean
}

export function CollapsibleChatText(props: CollapsibleChatTextProps) {
  return <MemoizedCollapsibleChatText {...props} />
}

const MemoizedCollapsibleChatText = memo(function MemoizedCollapsibleChatText({
  content,
  maxLines = 9,
  hideTaskCard = false,
  onTaskClick,
  isCustomerRoom,
  className,
  markdownClassName,
  expandLabel,
  collapseLabel,

}: CollapsibleChatTextProps) {
  const { t } = useLanguage()
  const [expanded, setExpanded] = useState(false)

  const bodyRef = useRef<HTMLDivElement>(null)
  const regionId = useId()
  const [limit, setLimit] = useState(216)
  const [isLong, setIsLong] = useState(false)
  useEffect(() => {
    const body = bodyRef.current
    if (!body) return
    let disposed = false
    const measure = () => {
      if (disposed) return
      const paragraph = body.querySelector('p, li') || body
      const lineHeight = parseFloat(getComputedStyle(paragraph).lineHeight) || 24
      const nextLimit = Math.max(1, maxLines) * lineHeight
      setLimit(nextLimit)
      // Legacy embedded cards must remain visible; normal attachments are siblings.
      setIsLong(!body.querySelector('[data-card]') && body.scrollHeight > nextLimit + 1)
    }
    const observer = new ResizeObserver(measure)
    observer.observe(body)
    measure()
    void document.fonts?.ready.then(measure)
    return () => { disposed = true; observer.disconnect() }
  }, [content, maxLines])

  return (
    <div className={cn('min-w-0 max-w-full', className)}>
      <div
        className={cn(
          'relative',
          isLong && !expanded && 'overflow-hidden',
        )}
        id={regionId}
        style={isLong && !expanded ? { maxHeight: limit } : undefined}
      >
        <div ref={bodyRef}>
        <ChatMarkdown
          content={content}
          className={markdownClassName}
          hideTaskCard={hideTaskCard}
          onTaskClick={onTaskClick}
          isCustomerRoom={isCustomerRoom}
        />
        </div>
      </div>
      {isLong && <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-controls={regionId}
        className="mt-1 min-h-11 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline cursor-pointer"
      >
        <span>{expanded ? (collapseLabel || t('Show less')) : (expandLabel || t('Read more'))}</span>
        {expanded ? (
          <ChevronUp className="h-3 w-3" />
        ) : (
          <ChevronDown className="h-3 w-3" />
        )}
      </button>}
    </div>
  )
})
