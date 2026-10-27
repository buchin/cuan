import { useState, FormEvent } from 'react'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import { useAuth } from '../../context/AuthContext'
import {
  KeyRound,
  Lock,
  Eye,
  EyeOff,
  CheckCircle2,
  AlertCircle,
  X,
  RefreshCw,
} from 'lucide-react'

interface ChangePasswordModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: () => void
}

export function ChangePasswordModal({
  isOpen,
  onClose,
  onSuccess,
}: ChangePasswordModalProps) {
  const { changePassword } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showCurrent, setShowCurrent] = useState(false)
  const [showNew, setShowNew] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  if (!isOpen) return null

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!currentPassword) {
      setError('Enter your current password.')
      return
    }
    if (newPassword.length < 8) {
      setError('Use at least 8 characters for your new password.')
      return
    }
    if (newPassword === currentPassword) {
      setError('Your new password must differ from your current password.')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }

    setLoading(true)
    try {
      await changePassword(currentPassword, newPassword)
      setSuccess(true)
      setTimeout(() => {
        setSuccess(false)
        setCurrentPassword('')
        setNewPassword('')
        setConfirmPassword('')
        onSuccess?.()
        onClose()
      }, 1500)
    } catch (err: any) {
      setError(
        err.message ||
          'Could not change your password. Check your current password and try again.',
      )
    } finally {
      setLoading(false)
    }
  }

  const handleClose = () => {
    if (loading) return
    setError(null)
    setSuccess(false)
    setCurrentPassword('')
    setNewPassword('')
    setConfirmPassword('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
      <Card className="w-full max-w-md bg-card text-card-foreground border-border shadow-2xl relative">
        <button
          onClick={handleClose}
          type="button"
          disabled={loading}
          className="absolute top-4 right-4 text-muted-foreground hover:text-foreground transition-colors"
          aria-label="Close"
        >
          <X className="h-5 w-5" />
        </button>

        <CardHeader className="space-y-1">
          <div className="inline-flex items-center gap-2 text-primary mb-1">
            <KeyRound className="h-5 w-5" />
            <span className="text-xs font-mono font-bold tracking-wider uppercase">
              Account security
            </span>
          </div>
          <CardTitle className="text-lg font-bold">Change password</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Update your Cuan password. Use the new password the next time you
            sign in.
          </CardDescription>
        </CardHeader>

        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>Could not update password</AlertTitle>
                <AlertDescription className="text-xs">{error}</AlertDescription>
              </Alert>
            )}

            {success && (
              <Alert className="border-emerald-500/50 bg-emerald-500/10 text-emerald-400">
                <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                <AlertTitle>Success!</AlertTitle>
                <AlertDescription className="text-xs">
                  Your password has been updated.
                </AlertDescription>
              </Alert>
            )}

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-foreground">
                Current password
              </label>
              <div className="relative">
                <Input
                  type={showCurrent ? 'text' : 'password'}
                  placeholder="••••••••••••"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  disabled={loading || success}
                  required
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowCurrent(!showCurrent)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showCurrent ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-foreground">
                New password (at least 8 characters)
              </label>
              <div className="relative">
                <Input
                  type={showNew ? 'text' : 'password'}
                  placeholder="••••••••••••"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  disabled={loading || success}
                  required
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowNew(!showNew)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showNew ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-foreground">
                Confirm new password
              </label>
              <div className="relative">
                <Input
                  type={showConfirm ? 'text' : 'password'}
                  placeholder="••••••••••••"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  disabled={loading || success}
                  required
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowConfirm(!showConfirm)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showConfirm ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>

            <div className="p-3 bg-muted/40 border border-border text-[11px] text-muted-foreground flex items-start gap-2">
              <Lock className="h-3.5 w-3.5 text-primary shrink-0 mt-0.5" />
              <span>Your password is stored as a bcrypt hash.</span>
            </div>
          </CardContent>

          <CardFooter className="flex items-center justify-end gap-2 pt-2 border-t border-border">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleClose}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              size="sm"
              disabled={loading || success}
              className="bg-primary text-primary-foreground hover:bg-primary/90"
            >
              {loading ? (
                <>
                  <RefreshCw className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                  Saving…
                </>
              ) : (
                'Save password'
              )}
            </Button>
          </CardFooter>
        </form>
      </Card>
    </div>
  )
}
