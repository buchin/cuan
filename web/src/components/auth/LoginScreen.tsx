import { Link } from 'react-router-dom'
import { CuanMark } from '../brand/CuanMark'
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
  Lock,
  User,
  Eye,
  EyeOff,
  ShieldCheck,
  AlertCircle,
  RefreshCw,
  Key,
} from 'lucide-react'

interface LoginScreenProps {
  onNavigate?: (path: string) => void
}

export function LoginScreen({ onNavigate: _onNavigate }: LoginScreenProps) {
  const { login, isLoading } = useAuth()
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!identifier.trim()) {
      setError("Enter the owner's username or email.")
      return
    }
    if (!password) {
      setError('Enter your password.')
      return
    }

    setSubmitting(true)
    try {
      await login(identifier, password)
      // Login successful! AuthContext updates state, which triggers AuthGate/App to render the authenticated shell.
    } catch (err: any) {
      setError(err.message || 'Incorrect username or password. Try again.')
    } finally {
      setSubmitting(false)
    }
  }

  const handleFillDefaultAdmin = () => {
    setIdentifier('admin')
    setPassword('whosyourdaddy')
    setError(null)
  }

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col justify-between p-4 sm:p-6 transition-colors duration-200">
      {/* Top Bar with Brand & Theme Toggle */}
      <header className="w-full max-w-5xl mx-auto flex items-center justify-between py-2">
        <Link
          to="/"
          className="flex min-h-11 items-center gap-2.5"
          aria-label="Cuan home"
        >
          <CuanMark className="h-9 w-9" />
          <span className="font-bold text-lg tracking-tight text-foreground">
            cuan
          </span>
        </Link>
      </header>

      {/* Main Login Card */}
      <main className="w-full max-w-md mx-auto my-auto py-8">
        <div className="space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-primary/10 text-primary border border-primary/20 mb-1">
              <Lock className="h-7 w-7" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Sign in to Cuan
            </h1>
            <p className="text-xs text-muted-foreground max-w-xs mx-auto">
              Manage your projects, conversations, and agents in one workspace.
            </p>
          </div>

          <Card className="bg-card text-card-foreground border-border shadow-2xl">
            <CardHeader className="space-y-1 pb-4 border-b border-border">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-semibold">
                  Owner sign-in
                </CardTitle>
                <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-1.5 py-0.5">
                  Single-Owner
                </span>
              </div>
              <CardDescription className="text-xs text-muted-foreground">
                Sign in with your configured owner account.
              </CardDescription>
            </CardHeader>

            <form onSubmit={handleSubmit}>
              <CardContent className="space-y-4 pt-4">
                {error && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Sign-in failed</AlertTitle>
                    <AlertDescription className="text-xs">
                      {error}
                    </AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-foreground flex items-center justify-between">
                    <span>Username or email</span>
                  </label>
                  <div className="relative">
                    <Input
                      type="text"
                      placeholder="admin or owner@example.com"
                      value={identifier}
                      onChange={(e) => setIdentifier(e.target.value)}
                      disabled={submitting || isLoading}
                      required
                      autoFocus
                      className="pl-9"
                    />
                    <User className="h-4 w-4 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold text-foreground">
                      Password
                    </label>
                    <button
                      type="button"
                      onClick={handleFillDefaultAdmin}
                      className="text-[11px] text-primary hover:underline flex items-center gap-1 font-mono"
                      title="Fill in the default admin credentials"
                    >
                      <Key className="h-3 w-3" />
                      Use default admin
                    </button>
                  </div>
                  <div className="relative">
                    <Input
                      type={showPassword ? 'text' : 'password'}
                      placeholder="••••••••••••"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      disabled={submitting || isLoading}
                      required
                      className="pl-9 pr-10"
                    />
                    <Lock className="h-4 w-4 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    >
                      {showPassword ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>

                {/* Default Credentials Callout */}
                <div className="p-3 bg-muted/50 border border-border text-xs text-muted-foreground space-y-1.5">
                  <div className="flex items-center gap-1.5 font-medium text-foreground text-[11px]">
                    <ShieldCheck className="h-3.5 w-3.5 text-primary" />
                    Default setup credentials
                  </div>
                  <p className="text-[11px]">
                    Username:{' '}
                    <code className="text-primary font-mono font-bold">
                      admin
                    </code>{' '}
                    | Password:{' '}
                    <code className="text-primary font-mono font-bold">
                      whosyourdaddy
                    </code>
                  </p>
                </div>
              </CardContent>

              <CardFooter className="pt-2 flex flex-col gap-2 border-t border-border">
                <Button
                  type="submit"
                  className="w-full bg-primary text-primary-foreground hover:bg-primary/90 font-semibold"
                  disabled={submitting || isLoading}
                >
                  {submitting ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Signing in…
                    </>
                  ) : (
                    'Sign in'
                  )}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </main>

      {/* Footer */}
      <footer className="w-full max-w-5xl mx-auto py-3 text-center text-xs text-muted-foreground border-t border-border/60">
        <div className="flex flex-col sm:flex-row items-center justify-between gap-2">
          <span>Cuan</span>
          <span className="font-mono text-[11px]">
            PostgreSQL Session Guard • Bcrypt Hash
          </span>
        </div>
      </footer>
    </div>
  )
}
