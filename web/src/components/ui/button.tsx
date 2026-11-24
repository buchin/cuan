import * as React from "react"
import { cn } from "../../lib/utils"

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "destructive" | "outline" | "secondary" | "ghost" | "link"
  size?: "default" | "sm" | "lg" | "icon" | "icon-sm"
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => {
    const baseStyles = "inline-flex items-center justify-center rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none select-none cursor-pointer"
    const variantStyles = {
      default: "bg-primary text-primary-foreground hover:bg-primary/90 shadow-xs",
      destructive: "bg-destructive/10 text-destructive border border-destructive/20 hover:bg-destructive/20",
      outline: "border border-input bg-background hover:bg-accent hover:text-accent-foreground text-foreground",
      secondary: "border border-input bg-background hover:bg-accent hover:text-accent-foreground text-foreground shadow-xs",
      ghost: "hover:bg-accent hover:text-accent-foreground text-foreground",
      link: "underline-offset-4 hover:underline text-primary",
    }
    const sizeStyles = {
      default: "h-9 py-2 px-4 text-sm gap-2",
      sm: "h-8 px-3 text-xs gap-1.5",
      lg: "h-11 px-6 text-sm gap-2",
      icon: "h-9 w-9 p-0",
      "icon-sm": "h-7 w-7 p-0",
    }

    return (
      <button
        className={cn(baseStyles, variantStyles[variant], sizeStyles[size], className)}
        ref={ref}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button }

