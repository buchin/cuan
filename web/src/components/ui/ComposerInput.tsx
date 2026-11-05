import { forwardRef, type TextareaHTMLAttributes } from 'react'

/** Shared writing surface for Inbox and terminal; transport stays with callers. */
export const ComposerInput = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function ComposerInput({ className = '', ...props }, ref) {
    return <textarea {...props} ref={ref} className={`wa-input min-w-0 flex-1 min-h-[44px] max-h-[140px] resize-none overflow-hidden py-3 text-[15px] sm:text-sm leading-5 pl-1.5 pr-2 sm:pl-2 sm:pr-2.5 ${className}`} />
  },
)

export function resizeComposerInput(input: HTMLTextAreaElement): boolean {
  input.style.height = 'auto'
  const minimum = 44
  const height = input.value.length ? input.scrollHeight : minimum
  const cap = Math.max(minimum, Math.min(140, Math.max(120, (window.visualViewport?.height || window.innerHeight) * 0.35)))
  const next = Math.min(Math.max(height, minimum), cap)
  input.style.height = `${next}px`
  input.style.overflowY = height > cap ? 'auto' : 'hidden'
  return next > minimum + 4
}
