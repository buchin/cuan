import { useEffect, useState, FormEvent } from 'react'
import { StudioDashboard } from './components/studio/StudioDashboard'
import { TermsPage, PrivacyPage } from './components/legal/LegalPages'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { Sidebar07Layout, ProjectOption } from './components/layout/Sidebar07Layout'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './components/ui/card'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import {
  Lock,
  RefreshCw,
  AlertCircle,
  ShieldCheck,
  CheckCircle2
} from 'lucide-react'

export function App() {
  const [currentPath, setCurrentPath] = useState<string>(() => window.location.pathname || '/')

  useEffect(() => {
    const handlePopState = () => {
      setCurrentPath(window.location.pathname || '/')
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const navigate = (path: string) => {
    window.history.pushState({}, '', path)
    setCurrentPath(path)
  }

  // Consume Auth Context
  const {
    token,
    isAuthenticated,
    isLoading: authLoading,
    setupRequired: authSetupRequired,
    refreshUser,
  } = useAuth()

  // System Info State
  const [systemMode, setSystemMode] = useState<{
    version: string
    base_domain: string
  }>({
    version: '0.1.0',
    base_domain: 'cuan.dojo.cc',
  })

  // Setup Form states (for fresh first-time instances)
  const [setupLoading, setSetupLoading] = useState(false)
  const [setupError, setSetupError] = useState<string | null>(null)
  const [setupEmail, setSetupEmail] = useState('')
  const [setupPassword, setSetupPassword] = useState('')
  const [setupConfirmPassword, setSetupConfirmPassword] = useState('')

  // Notifications
  const [successNotification, setSuccessNotification] = useState<string | null>(null)

  // Project List for Workspace Switcher (S15.3: defaults to empty for true 0-project empty state)
  const [projectsList, setProjectsList] = useState<ProjectOption[]>([])
  const [activeProject, setActiveProject] = useState<ProjectOption | undefined>(undefined)
  const [navMode, setNavMode] = useState<'global' | 'project'>('global')

  const handleSelectProject = (proj: ProjectOption) => {
    setActiveProject(proj)
    setSuccessNotification(`Beralih ke workspace "${proj.name}"`)
  }

  const handleCreateProject = async (projectData: any) => {
    setProjectsList((prev) => [projectData, ...prev])
    setActiveProject(projectData)
    setNavMode('project')
    setSuccessNotification(`Proyek baru "${projectData.name}" berhasil dibuat!`)
  }

  useEffect(() => {
    const fetchSystemInfo = async () => {
      try {
        const [modeRes, projectsRes] = await Promise.allSettled([
          fetch('/api/system/mode'),
          fetch('/api/v1/projects', {
            headers: token ? { Authorization: `Bearer ${token}` } : {},
          }),
        ])

        if (modeRes.status === 'fulfilled' && modeRes.value.ok) {
          const modeData = await modeRes.value.json()
          setSystemMode({
            version: modeData.version || '0.1.0',
            base_domain: modeData.base_domain || 'cuan.dojo.cc',
          })
        }

        if (projectsRes.status === 'fulfilled' && projectsRes.value.ok) {
          const pData = await projectsRes.value.json()
          if (pData.projects && Array.isArray(pData.projects)) {
            const mapped: ProjectOption[] = pData.projects
              .filter((p: any) => !p.is_archived && p.status !== 'archived')
              .map((p: any) => ({
                id: p.id,
                name: p.name || `${p.repo_owner || 'repo'}/${p.repo_name || p.id}`,
                type: p.project_type || 'saas',
                status: p.is_archived ? 'archived' : 'active',
                is_archived: p.is_archived,
              }))
            setProjectsList(mapped)
            if (mapped.length > 0) {
              setActiveProject(mapped[0])
            }
          }
        }
      } catch (err) {
        console.warn('System status fetch info:', err)
      }
    }

    fetchSystemInfo()
  }, [token])

  // First-time owner setup submit
  const handleSetupSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSetupError(null)

    if (setupPassword.length < 8) {
      setSetupError('Kata sandi harus minimal 8 karakter.')
      return
    }
    if (setupPassword !== setupConfirmPassword) {
      setSetupError('Konfirmasi kata sandi tidak cocok.')
      return
    }

    setSetupLoading(true)
    try {
      const res = await fetch('/api/v1/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: setupEmail.trim(), password: setupPassword }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Setup gagal dilakukan')
      }

      if (data.token) {
        localStorage.setItem('cuan_token', data.token)
      }
      await refreshUser()
    } catch (err: any) {
      setSetupError(err.message || 'Terjadi kesalahan saat setup pemilik.')
    } finally {
      setSetupLoading(false)
    }
  }

  // 1. PUBLIC ROUTES (Accessible without authentication)

  // Legal routes (/terms & /privacy - S03.5)
  if (currentPath === '/terms') {
    return <TermsPage onNavigate={navigate} />
  }

  if (currentPath === '/privacy') {
    return <PrivacyPage onNavigate={navigate} />
  }

  // Explicit Login Route
  if (currentPath === '/login') {
    if (isAuthenticated) {
      navigate('/dashboard')
      return null
    }
    return <LoginScreen onNavigate={navigate} />
  }

  // 2. LOADING STATE
  if (authLoading && authSetupRequired === null) {
    return (
      <div className="min-h-screen bg-background text-foreground flex items-center justify-center p-4">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 text-primary animate-spin" />
          <p className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">
            Memeriksa Sesi Cuan...
          </p>
        </div>
      </div>
    )
  }

  // 3. FIRST-TIME SETUP SCREEN (WHEN DATABASE HAS NO OWNER)
  if (authSetupRequired) {
    return (
      <div className="min-h-screen bg-background text-foreground flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-primary/10 text-primary border border-primary/20 mb-1">
              <ShieldCheck className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Setup Pertama Pemilik Cuan
            </h1>
            <p className="text-xs text-muted-foreground">
              Instance Cuan beroperasi dalam mode single-owner. Akun pertama ini menjadi pengelola tunggal instance.
            </p>
          </div>

          <Card className="bg-card text-card-foreground border-border shadow-2xl">
            <CardHeader className="space-y-1 pb-4 border-b border-border">
              <CardTitle className="text-sm font-semibold">Buat Akun Pemilik</CardTitle>
              <CardDescription className="text-xs text-muted-foreground">
                Setelah pendaftaran berhasil, registrasi publik ditutup permanen untuk menjaga keamanan platform.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleSetupSubmit}>
              <CardContent className="space-y-4 pt-4">
                {setupError && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Gagal Setup</AlertTitle>
                    <AlertDescription className="text-xs">{setupError}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-foreground">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={setupEmail}
                    onChange={(e) => setSetupEmail(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-foreground">Kata Sandi (min. 8 karakter)</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={setupPassword}
                    onChange={(e) => setSetupPassword(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-foreground">Konfirmasi Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={setupConfirmPassword}
                    onChange={(e) => setSetupConfirmPassword(e.target.value)}
                    required
                  />
                </div>

                <div className="p-3 bg-muted/40 border border-border text-[11px] text-muted-foreground space-y-1">
                  <div className="flex items-center gap-1.5 font-medium text-foreground">
                    <Lock className="h-3.5 w-3.5 text-primary" />
                    Keamanan Tokyo Night & PostgreSQL
                  </div>
                  <p>
                    Data tersimpan secara aman pada volume PostgreSQL. Kredensial di-hash dengan bcrypt standar keamanan tinggi.
                  </p>
                </div>
              </CardContent>
              <CardFooter className="pt-2 flex flex-col gap-2 border-t border-border">
                <Button
                  type="submit"
                  className="w-full bg-primary text-primary-foreground hover:bg-primary/90 font-semibold"
                  disabled={setupLoading}
                >
                  {setupLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Menyimpan Akun Pemilik...
                    </>
                  ) : (
                    'Inisialisasi & Amankan Instance'
                  )}
                </Button>
                <button
                  type="button"
                  onClick={() => navigate('/login')}
                  className="text-xs text-muted-foreground hover:text-foreground underline pt-1"
                >
                  ← Kembali ke Halaman Login
                </button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 4. AUTHENTICATION GATE (WHEN NOT LOGGED IN)
  if (!isAuthenticated) {
    return <LoginScreen onNavigate={navigate} />
  }

  // 5. AUTHENTICATED MODERN SHELL: SHADCN SIDEBAR-07 & TOKYO NIGHT THEME
  return (
    <Sidebar07Layout
      currentPath={currentPath}
      onNavigate={navigate}
      projects={projectsList.filter((p) => !p.is_archived && p.status !== 'archived')}
      activeProject={activeProject}
      onSelectProject={handleSelectProject}
      onCreateProject={handleCreateProject}
      navMode={navMode}
      onToggleNavMode={setNavMode}
    >
      {/* Global Success Notification */}
      {successNotification && (
        <div className="mb-4 p-3 bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-emerald-400" />
            <span className="font-medium">{successNotification}</span>
          </div>
          <button
            onClick={() => setSuccessNotification(null)}
            className="text-emerald-400 hover:text-emerald-200 font-bold"
            aria-label="Tutup notifikasi"
          >
            ✕
          </button>
        </div>
      )}

      {/* Main Content Area */}
      <StudioDashboard
        currentPath={
          currentPath.startsWith('/studio')
            ? currentPath
            : currentPath === '/' || currentPath === '/dashboard'
            ? '/studio'
            : `/studio${currentPath}`
        }
        onNavigate={navigate}
        baseDomain={systemMode.base_domain}
        hideHeader={true}
      />
    </Sidebar07Layout>
  )
}

export default App
