import * as React from 'react'
import { cn } from '../../lib/utils'

export interface FormFieldProps {
  id?: string
  label?: React.ReactNode
  accessory?: React.ReactNode
  helperText?: React.ReactNode
  error?: React.ReactNode
  required?: boolean
  htmlFor?: string
  className?: string
  labelClassName?: string
  children: React.ReactNode
}

export function FormField({
  id,
  label,
  accessory,
  helperText,
  error,
  required,
  htmlFor,
  className,
  labelClassName,
  children,
}: FormFieldProps) {
  const targetId = htmlFor || id

  return (
    <div className={cn('space-y-1.5', className)}>
      {(label || accessory) && (
        <div className="flex items-center justify-between gap-2">
          {label && (
            <label
              htmlFor={targetId}
              className={cn(
                'text-xs font-semibold uppercase tracking-wider text-muted-foreground',
                labelClassName,
              )}
            >
              {label}
              {required && <span className="ml-1 text-destructive">*</span>}
            </label>
          )}
          {accessory && (
            <div className="text-[11px] text-muted-foreground">{accessory}</div>
          )}
        </div>
      )}
      {children}
      {error ? (
        <p className="text-[11px] text-destructive leading-normal">{error}</p>
      ) : helperText ? (
        <p className="text-[11px] text-muted-foreground leading-normal">{helperText}</p>
      ) : null}
    </div>
  )
}
