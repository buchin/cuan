import { useEffect, useState, FormEvent } from 'react'
import { LandingPage } from './components/landing/LandingPage'
import { SponsorShowcase } from './components/sponsor/SponsorShowcase'
import { SponsorArchive } from './components/sponsor/SponsorArchive'
import { SponsorPending } from './components/sponsor/SponsorPending'
import { AdminSponsorPanel } from './components/sponsor/AdminSponsorPanel'
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
    user,
    token,
    isAuthenticated,
    isLoading: authLoading,
    setupRequired: authSetupRequired,
    refreshUser,
  } = useAuth()

  // System Mode State (APP_MODE=marketing vs APP_MODE=studio - S03.5)
  const [systemMode, setSystemMode] = useState<{
    app_mode: 'marketing' | 'studio'
    version: string
    base_domain: string
  }>({
    app_mode: 'marketing',
    version: '0.1.0',
    base_domain: 'cuan.dojo.cc',
  })

  const [overrideMode, setOverrideMode] = useState<'marketing' | 'studio' | null>(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('cuan_app_mode_override')
      return saved === 'marketing' || saved === 'studio' ? saved : null
    }
    return null
  })

  const effectiveMode: 'marketing' | 'studio' = overrideMode || systemMode.app_mode

  const handleToggleMode = (newMode: 'marketing' | 'studio') => {
    setOverrideMode(newMode)
    localStorage.setItem('cuan_app_mode_override', newMode)
  }

  // Setup Form states (for fresh first-time instances)
  const [setupLoading, setSetupLoading] = useState(false)
  const [setupError, setSetupError] = useState<string | null>(null)
  const [setupEmail, setSetupEmail] = useState('')
  const [setupPassword, setSetupPassword] = useState('')
  const [setupConfirmPassword, setSetupConfirmPassword] = useState('')

  // Notifications
  const [successNotification, setSuccessNotification] = useState<string | null>(null)

  // Project List for Workspace Switcher
  const [projectsList, setProjectsList] = useState<ProjectOption[]>([
    { id: 'cuan-core', name: 'Cuan Platform Core', type: 'saas', status: 'active' },
    { id: 'saas-dojo', name: 'SaaS / Dojo Workspace', type: 'saas', status: 'active' },
    { id: 'creator-hub', name: 'Creator Hub Workspace', type: 'creator', status: 'active' },
    { id: 'blank-workspace', name: 'Blank Workspace', type: 'blank', status: 'active' },
  ])
  const [activeProject, setActiveProject] = useState<ProjectOption>(() => projectsList[0])
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
          setSystemMode(modeData)
        }

        if (projectsRes.status === 'fulfilled' && projectsRes.value.ok) {
          const pData = await projectsRes.value.json()
          if (pData.projects && Array.isArray(pData.projects) && pData.projects.length > 0) {
            const mapped = pData.projects.map((p: any) => ({
              id: p.id,
              name: p.name || `${p.repo_owner || 'repo'}/${p.repo_name || p.id}`,
              type: p.project_type || 'saas',
              status: 'active',
            }))
            setProjectsList((prev) => {
              const existingIds = new Set(mapped.map((m: any) => m.id))
              return [...mapped, ...prev.filter((item) => !existingIds.has(item.id))]
            })
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

  // Explicit Marketing Landing Page route (/marketing & /landing)
  if (currentPath === '/marketing' || currentPath === '/landing') {
    return (
      <LandingPage
        onNavigate={navigate}
        appMode={effectiveMode}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Sponsorship Pending Confirmation Route
  if (
    currentPath === '/sponsor/pending' ||
    currentPath === '/sponsors/pending' ||
    currentPath === '/sponsor/status' ||
    (currentPath === '/sponsor' && window.location.search.includes('status='))
  ) {
    return <SponsorPending onNavigate={navigate} />
  }

  // Sponsorship Showcase Route (/sponsor)
  if (currentPath === '/sponsor') {
    return <SponsorShowcase onNavigate={navigate} />
  }

  // Sponsorship Archive & Directory Route (/sponsors and /sponsors/:batch)
  if (currentPath === '/sponsors' || currentPath.startsWith('/sponsors/')) {
    return <SponsorArchive onNavigate={navigate} />
  }

  // Root ('/' and '/home') marketing mode
  if ((currentPath === '/' || currentPath === '/home') && effectiveMode === 'marketing') {
    return (
      <LandingPage
        onNavigate={navigate}
        appMode={effectiveMode}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Explicit Login Route
  if (currentPath === '/login') {
    if (isAuthenticated) {
      navigate('/studio')
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
                  onClick={() => navigate('/')}
                  className="text-xs text-muted-foreground hover:text-foreground underline pt-1"
                >
                  ← Kembali ke Landing Page
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
      appMode={effectiveMode}
      onToggleMode={handleToggleMode}
      projects={projectsList}
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
      {currentPath === '/admin/sponsors' ? (
        <AdminSponsorPanel
          token={token}
          onNavigate={navigate}
          onNotify={(msg) => setSuccessNotification(msg)}
          ownerEmail={user?.email || 'admin@cuan.local'}
        />
      ) : (
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
          appMode={effectiveMode}
          isStudioModeActive={effectiveMode === 'studio'}
          onToggleMode={handleToggleMode}
          hideHeader={true}
        />
      )}
    </Sidebar07Layout>
  )
}

export default App
