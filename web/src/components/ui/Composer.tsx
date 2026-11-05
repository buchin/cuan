import type { ComponentProps } from 'react'
import { cn } from '../../lib/utils'
import './composer.css'

/** Shared layout only: each caller owns text, uploads and its send transport. */
export function Composer({ className, ...props }: ComponentProps<'div'>) {
  return <div {...props} className={cn('wa-input-capsule shared-composer flex min-w-0 items-end gap-1 sm:gap-1.5 pl-1 sm:pl-1.5', className)} />
}

/** A stable bottom-aligned44px row centers small accessory icons on one line. */
export function ComposerAccessory({ className, ...props }: ComponentProps<'div'>) {
  return <div {...props} className={cn('flex h-11 min-w-11 shrink-0 items-center justify-center', className)} />
}

export function ComposerSendButton({ className, children, ...props }: ComponentProps<'button'>) {
  return <button type="button" {...props} className={cn('composer-send flex w-12 min-h-11 shrink-0 self-stretch items-end justify-center transition-colors disabled:opacity-40 disabled:pointer-events-none cursor-pointer focus-visible:outline-2 focus-visible:outline-ring', className)}><span className="flex h-11 w-full items-center justify-center pointer-events-none">{children}</span></button>
}
