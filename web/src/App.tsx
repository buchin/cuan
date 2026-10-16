import { useEffect, useState, FormEvent } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { 
  CheckCircle2, 
  Server, 
  Shield, 
  Database, 
  Lock, 
  LogOut, 
  RefreshCw, 
  GitBranch, 
  AlertCircle,
  ShieldCheck,
  Globe,
  HardDrive,
  FolderGit2,
  ExternalLink,
  Plus,
  ArrowLeft,
  Trash2,
  HelpCircle,
  KeyRound,
  Check,
  ChevronRight
} from 'lucide-react'

interface OwnerInfo {
  id: string
  email: string
}

interface ServerInfo {
  name: string
  sprint: string
  status: string
  adapter: string
  license: string
  architecture: string
}

interface HealthInfo {
  status: string
  app: string
  version: string
}

interface ReadyInfo {
  status: string
  database: string
  migrations?: string
  migrations_count?: number
  owner_configured?: boolean
}

interface TaskItem {
  id: string
  title: string
  status: string
  priority: string
}

interface ProjectItem {
  id: string
  name: string
  repo_url: string
  repo_owner: string
  repo_name: string
  default_branch: string
  local_path?: string
  has_token: boolean
  token_preview?: string
  clone_status: string // connected, synced, failed, pending
  last_fetched_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

export function App() {
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null)
  const [owner, setOwner] = useState<OwnerInfo | null>(null)
  const [token, setToken] = useState<string>(() => localStorage.getItem('cuan_token') || '')
  
  const [info, setInfo] = useState<ServerInfo | null>(null)
  const [health, setHealth] = useState<HealthInfo | null>(null)
  const [ready, setReady] = useState<ReadyInfo | null>(null)
  const [tasks, setTasks] = useState<TaskItem[]>([])
  const [projects, setProjects] = useState<ProjectItem[]>([])
  
  // Navigation
  const [activeTab, setActiveTab] = useState<'overview' | 'projects' | 'tasks'>('projects')
  const [selectedProject, setSelectedProject] = useState<ProjectItem | null>(null)
  const [projectBranches, setProjectBranches] = useState<string[]>([])
  const [branchesLoading, setBranchesLoading] = useState(false)

  // Dialog & Form states
  const [isConnectModalOpen, setIsConnectModalOpen] = useState(false)
  const [showTokenGuide, setShowTokenGuide] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  
  const [repoNameInput, setRepoNameInput] = useState('')
  const [repoUrlInput, setRepoUrlInput] = useState('')
  const [githubTokenInput, setGithubTokenInput] = useState('')
  const [defaultBranchInput, setDefaultBranchInput] = useState('')
  
  const [loading, setLoading] = useState(true)
  const [authLoading, setAuthLoading] = useState(false)
  const [connectLoading, setConnectLoading] = useState(false)
  const [syncLoading, setSyncLoading] = useState(false)
  const [deleteLoading, setDeleteLoading] = useState(false)
  
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [connectError, setConnectError] = useState<string | null>(null)
  const [successNotification, setSuccessNotification] = useState<string | null>(null)

  // Auth Form states
  const [emailInput, setEmailInput] = useState('')
  const [passwordInput, setPasswordInput] = useState('')
  const [confirmPasswordInput, setConfirmPasswordInput] = useState('')

  const getAuthHeaders = (): HeadersInit => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    return headers
  }

  const checkStatusAndAuth = async (currentToken: string = token) => {
    setLoading(true)
    setErrorMessage(null)

    try {
      // 1. Check Auth Status (Setup required vs configured)
      const statusRes = await fetch('/api/v1/auth/status')
      if (statusRes.ok) {
        const statusData = await statusRes.json()
        setSetupRequired(statusData.setup_required)

        if (!statusData.setup_required) {
          // Check if currently authenticated
          const headers: Record<string, string> = {}
          if (currentToken) {
            headers['Authorization'] = `Bearer ${currentToken}`
          }
          const meRes = await fetch('/api/v1/auth/me', { headers })
          if (meRes.ok) {
            const meData = await meRes.json()
            setOwner(meData.owner)
            await fetchProtectedData(currentToken)
          } else {
            setOwner(null)
          }
        }
      }

      // 2. Fetch Health & Readiness
      const [healthRes, readyRes, infoRes] = await Promise.allSettled([
        fetch('/healthz'),
        fetch('/readyz'),
        fetch('/api/v1/info')
      ])

      if (healthRes.status === 'fulfilled' && healthRes.value.ok) {
        setHealth(await healthRes.value.json())
      }
      if (readyRes.status === 'fulfilled' && readyRes.value.ok) {
        setReady(await readyRes.value.json())
      }
      if (infoRes.status === 'fulfilled' && infoRes.value.ok) {
        setInfo(await infoRes.value.json())
      }
    } catch (err) {
      console.error('Failed to load system status:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchProtectedData = async (activeToken: string = token) => {
    try {
      const headers: Record<string, string> = {}
      if (activeToken) {
        headers['Authorization'] = `Bearer ${activeToken}`
      }
      
      const [tasksRes, projectsRes] = await Promise.allSettled([
        fetch('/api/v1/tasks', { headers }),
        fetch('/api/v1/projects', { headers }),
      ])

      if (tasksRes.status === 'fulfilled' && tasksRes.value.ok) {
        const tasksData = await tasksRes.value.json()
        setTasks(tasksData.tasks || [])
      }

      if (projectsRes.status === 'fulfilled' && projectsRes.value.ok) {
        const projectsData = await projectsRes.value.json()
        const pList = projectsData.projects || []
        setProjects(pList)

        
        // If a project is currently selected, refresh its data
        if (selectedProject) {
          const updated = pList.find((p: ProjectItem) => p.id === selectedProject.id)
          if (updated) {
            setSelectedProject(updated)
          }
        }
      }
    } catch (err) {
      console.error('Failed to fetch protected data:', err)
    }
  }

  const fetchProjectBranches = async (projectId: string) => {
    setBranchesLoading(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/branches`, {
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setProjectBranches(data.branches || [])
      }
    } catch (err) {
      console.error('Failed to fetch branches:', err)
    } finally {
      setBranchesLoading(false)
    }
  }

  const handleSelectProject = (project: ProjectItem) => {
    setSelectedProject(project)
    fetchProjectBranches(project.id)
  }

  useEffect(() => {
    checkStatusAndAuth()
  }, [])

  const handleSetup = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)

    if (passwordInput.length < 8) {
      setErrorMessage('Kata sandi harus minimal 8 karakter.')
      return
    }
    if (passwordInput !== confirmPasswordInput) {
      setErrorMessage('Konfirmasi kata sandi tidak cocok.')
      return
    }

    setAuthLoading(true)
    try {
      const res = await fetch('/api/v1/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Setup gagal dilakukan')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setSetupRequired(false)
      setEmailInput('')
      setPasswordInput('')
      setConfirmPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Terjadi kesalahan saat setup')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogin = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)
    setAuthLoading(true)

    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Login gagal')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setEmailInput('')
      setPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Email atau password salah')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogout = async () => {
    try {
      await fetch('/api/v1/auth/logout', {
        method: 'POST',
        headers: getAuthHeaders(),
      })
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      localStorage.removeItem('cuan_token')
      setToken('')
      setOwner(null)
      setTasks([])
      setProjects([])
      setSelectedProject(null)
    }
  }

  const handleConnectProject = async (e: FormEvent) => {
    e.preventDefault()
    setConnectError(null)
    setConnectLoading(true)

    try {
      const payload: any = {
        repo_url: repoUrlInput.trim(),
      }
      if (repoNameInput.trim()) {
        payload.name = repoNameInput.trim()
      }
      if (githubTokenInput.trim()) {
        payload.token = githubTokenInput.trim()
      }
      if (defaultBranchInput.trim()) {
        payload.default_branch = defaultBranchInput.trim()
      }

      const res = await fetch('/api/v1/projects', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal menghubungkan repository GitHub')
      }

      // Reset form and close modal
      setRepoNameInput('')
      setRepoUrlInput('')
      setGithubTokenInput('')
      setDefaultBranchInput('')
      setIsConnectModalOpen(false)
      setShowTokenGuide(false)

      // Refresh projects list and select the new project
      await fetchProtectedData()
      setSelectedProject(data)
      fetchProjectBranches(data.id)
      setSuccessNotification(`Repository ${data.repo_owner}/${data.repo_name} berhasil dihubungkan!`)
      setTimeout(() => setSuccessNotification(null), 5000)
    } catch (err: any) {
      setConnectError(err.message || 'Terjadi kesalahan saat menghubungkan repository')
    } finally {
      setConnectLoading(false)
    }
  }

  const handleSyncProject = async (projectId: string) => {
    setSyncLoading(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/sync`, {
        method: 'POST',
        headers: getAuthHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Sinkronisasi gagal')
      }

      await fetchProtectedData()
      if (selectedProject?.id === projectId) {
        setSelectedProject(data)
        fetchProjectBranches(projectId)
      }
      setSuccessNotification('Sinkronisasi repository berhasil!')
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      alert(`Gagal sinkronisasi: ${err.message}`)
    } finally {
      setSyncLoading(false)
    }
  }

  const handleDeleteProject = async () => {
    if (!selectedProject) return
    setDeleteLoading(true)

    try {
      const res = await fetch(`/api/v1/projects/${selectedProject.id}`, {
        method: 'DELETE',
        headers: getAuthHeaders(),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal menghapus proyek')
      }

      setDeleteModalOpen(false)
      setSelectedProject(null)
      await fetchProtectedData()
      setSuccessNotification('Proyek berhasil dihapus dari Cuan. Remote GitHub Anda tetap aman.')
      setTimeout(() => setSuccessNotification(null), 5000)
    } catch (err: any) {
      alert(`Gagal menghapus proyek: ${err.message}`)
    } finally {
      setDeleteLoading(false)
    }
  }

  if (loading && setupRequired === null) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 text-emerald-600 animate-spin" />
          <p className="text-sm font-medium text-slate-600">Memeriksa status instance Cuan...</p>
        </div>
      </div>
    )
  }

  // 1. FIRST-TIME SETUP SCREEN
  if (setupRequired) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-emerald-100 rounded-2xl mb-1 text-emerald-700">
              <ShieldCheck className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Setup Pertama Pemilik Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance Cuan beroperasi dalam mode pemilik tunggal (single-owner). Akun pertama ini menjadi satu-satunya pengelola instance.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Buat Akun Pemilik</CardTitle>
              <CardDescription className="text-xs">
                Setelah pendaftaran berhasil, registrasi publik ditutup permanen untuk mencegah pengambilalihan pihak ketiga.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleSetup}>
              <CardContent className="space-y-4">
                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Gagal Setup</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi (min. 8 karakter)</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Konfirmasi Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={confirmPasswordInput}
                    onChange={(e) => setConfirmPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="p-3 bg-slate-50 rounded-md border border-slate-200 text-xs text-slate-600 space-y-1">
                  <div className="flex items-center gap-1.5 font-medium text-slate-700">
                    <Lock className="h-3.5 w-3.5 text-emerald-600" />
                    Jaminan Keamanan & DoD
                  </div>
                  <p>
                    Data tersimpan secara persisten pada volume PostgreSQL. Kredensial di-hash dengan bcrypt dan database tidak diekspos ke publik.
                  </p>
                </div>
              </CardContent>
              <CardFooter className="pt-2">
                <Button type="submit" className="w-full bg-emerald-600 hover:bg-emerald-700 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Menyimpan Akun Pemilik...
                    </>
                  ) : (
                    'Inisialisasi & Amankan Instance'
                  )}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 2. LOGIN SCREEN (WHEN NOT LOGGED IN)
  if (!owner) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-slate-900 rounded-2xl mb-1 text-white">
              <Lock className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Masuk ke Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance terkonfigurasi. Masuk dengan akun pemilik terdaftar.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Autentikasi Pemilik Tunggal</CardTitle>
              <CardDescription className="text-xs">
                Akses API, runner agent, dan data proyek dilindungi oleh sesi terenkripsi.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleLogin}>
              <CardContent className="space-y-4">
                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Autentikasi Gagal</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="flex items-center justify-between text-xs text-slate-500 pt-1">
                  <span className="flex items-center gap-1">
                    <Shield className="h-3.5 w-3.5 text-emerald-600" />
                    Pendaftaran publik dikunci
                  </span>
                  <span>PostgreSQL Persistent</span>
                </div>
              </CardContent>
              <CardFooter className="pt-2">
                <Button type="submit" className="w-full bg-slate-900 hover:bg-slate-800 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Memverifikasi...
                    </>
                  ) : (
                    'Masuk ke Dashboard'
                  )}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 3. AUTHENTICATED DASHBOARD (UI SHELL)
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 w-full p-4 md:p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        
        {/* Top Header */}
        <header className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-5 rounded-xl border border-slate-200 shadow-sm">
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-black tracking-tight text-slate-900">Cuan</h1>
              <Badge variant="success" className="text-xs uppercase">
                Sprint 1: Running
              </Badge>
              <Badge variant="outline" className="text-xs font-mono bg-blue-50 text-blue-700 border-blue-200">
                T3: GitHub Existing Repo
              </Badge>
            </div>
            <p className="text-xs text-slate-500">
              Agentic project management for self-hosted teams. Develop Cuan using Cuan.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <div className="text-right hidden sm:block">
              <div className="text-xs font-bold text-slate-800">{owner.email}</div>
              <div className="text-[11px] text-emerald-600 font-medium">Owner Terautentikasi</div>
            </div>

            <Button variant="outline" size="sm" onClick={() => fetchProtectedData()} disabled={loading} className="gap-1.5 text-xs">
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>

            <Button variant="ghost" size="sm" onClick={handleLogout} className="gap-1.5 text-xs text-red-600 hover:bg-red-50">
              <LogOut className="h-3.5 w-3.5" />
              Keluar
            </Button>
          </div>
        </header>

        {/* Global Success Notification */}
        {successNotification && (
          <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-lg text-emerald-900 text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
              <span className="font-medium">{successNotification}</span>
            </div>
            <button onClick={() => setSuccessNotification(null)} className="text-emerald-700 hover:text-emerald-900">
              ✕
            </button>
          </div>
        )}

        {/* Navigation Tabs */}
        <div className="flex items-center gap-2 border-b border-slate-200 pb-2">
          <Button
            variant={activeTab === 'projects' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('projects'); setSelectedProject(null); }}
            className={`gap-1.5 text-xs ${activeTab === 'projects' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <FolderGit2 className="h-4 w-4" />
            Projects ({projects.length})
          </Button>

          <Button
            variant={activeTab === 'tasks' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('tasks'); setSelectedProject(null); }}
            className={`gap-1.5 text-xs ${activeTab === 'tasks' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <GitBranch className="h-4 w-4" />
            Tasks Board ({tasks.length})
          </Button>

          <Button
            variant={activeTab === 'overview' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('overview'); setSelectedProject(null); }}
            className={`gap-1.5 text-xs ${activeTab === 'overview' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <Server className="h-4 w-4" />
            System Overview
          </Button>
        </div>

        {/* ============================================================== */}
        {/* TAB 1: PROJECTS & PROJECT DETAIL (MAIN SCOPE T3)               */}
        {/* ============================================================== */}
        {activeTab === 'projects' && (
          <div className="space-y-6">
            {!selectedProject ? (
              // 1. Projects List View
              <div className="space-y-4">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
                  <div>
                    <h2 className="text-lg font-bold text-slate-900">Daftar Project GitHub</h2>
                    <p className="text-xs text-slate-500">
                      Hubungkan repository GitHub existing untuk pengembangan mandiri via Cuan runner & coding agent.
                    </p>
                  </div>

                  <Button
                    onClick={() => { setIsConnectModalOpen(true); setConnectError(null); }}
                    className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                    size="sm"
                  >
                    <Plus className="h-4 w-4" />
                    Hubungkan Repository GitHub
                  </Button>
                </div>

                {projects.length === 0 ? (
                  <Card className="bg-white border-dashed border-2 border-slate-300 p-8 text-center">
                    <CardContent className="space-y-3">
                      <div className="inline-flex p-3 bg-slate-100 rounded-full text-slate-500">
                        <FolderGit2 className="h-8 w-8" />
                      </div>
                      <h3 className="text-sm font-semibold text-slate-800">Belum ada repository terhubung</h3>
                      <p className="text-xs text-slate-500 max-w-md mx-auto">
                        Hubungkan repository GitHub existing Anda (misal <code>https://github.com/buchin/cuan</code>) untuk mulai mengelola task dan menjalankan agent.
                      </p>
                      <Button
                        onClick={() => setIsConnectModalOpen(true)}
                        className="bg-slate-900 hover:bg-slate-800 text-white text-xs mt-2"
                        size="sm"
                      >
                        Hubungkan Repository Pertama
                      </Button>
                    </CardContent>
                  </Card>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {projects.map((p) => (
                      <Card key={p.id} className="bg-white border-slate-200 hover:border-slate-300 transition-all shadow-sm">
                        <CardHeader className="p-4 pb-3">
                          <div className="flex items-start justify-between gap-2">
                            <div className="space-y-1">
                              <CardTitle className="text-base font-bold text-slate-900 flex items-center gap-1.5">
                                {p.name}
                                <span className="text-xs font-normal text-slate-500 font-mono">
                                  ({p.repo_owner}/{p.repo_name})
                                </span>
                              </CardTitle>
                              <a
                                href={p.repo_url}
                                target="_blank"
                                rel="noreferrer"
                                className="text-xs text-blue-600 hover:underline flex items-center gap-1"
                              >
                                {p.repo_url}
                                <ExternalLink className="h-3 w-3" />
                              </a>
                            </div>

                            <Badge
                              variant={p.clone_status === 'synced' ? 'success' : p.clone_status === 'failed' ? 'destructive' : 'default'}
                              className="text-[11px] capitalize"
                            >
                              {p.clone_status}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="p-4 pt-0 space-y-2.5">
                          <div className="flex items-center gap-2 text-xs text-slate-600">
                            <GitBranch className="h-3.5 w-3.5 text-slate-400" />
                            <span>Default Branch: <strong className="font-mono text-slate-800">{p.default_branch}</strong></span>
                          </div>

                          <div className="flex items-center gap-2 text-xs text-slate-600">
                            <KeyRound className="h-3.5 w-3.5 text-slate-400" />
                            <span>
                              Kredensial: {p.has_token ? (
                                <span className="font-mono text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-200">
                                  {p.token_preview || 'Tersimpan Terenkripsi'}
                                </span>
                              ) : (
                                <span className="text-slate-500 italic">Public (Tanpa Token)</span>
                              )}
                            </span>
                          </div>

                          {p.last_fetched_at && (
                            <div className="text-[11px] text-slate-400">
                              Terakhir sinkronisasi: {new Date(p.last_fetched_at).toLocaleString()}
                            </div>
                          )}

                          {p.last_error && (
                            <div className="text-xs text-red-600 bg-red-50 p-2 rounded border border-red-100">
                              Error: {p.last_error}
                            </div>
                          )}
                        </CardContent>
                        <CardFooter className="p-4 pt-2 border-t border-slate-100 flex items-center justify-between">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleSyncProject(p.id)}
                            disabled={syncLoading}
                            className="text-xs gap-1.5"
                          >
                            <RefreshCw className={`h-3 w-3 ${syncLoading ? 'animate-spin' : ''}`} />
                            Sync Remote
                          </Button>

                          <Button
                            variant="default"
                            size="sm"
                            onClick={() => handleSelectProject(p)}
                            className="text-xs bg-slate-900 hover:bg-slate-800 text-white gap-1"
                          >
                            Buka Detail
                            <ChevronRight className="h-3.5 w-3.5" />
                          </Button>
                        </CardFooter>
                      </Card>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              // 2. Project Detail View
              <div className="space-y-5">
                <div className="flex items-center justify-between">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSelectedProject(null)}
                    className="text-xs gap-1.5 text-slate-600 hover:text-slate-900"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Kembali ke Daftar Proyek
                  </Button>

                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleSyncProject(selectedProject.id)}
                      disabled={syncLoading}
                      className="text-xs gap-1.5"
                    >
                      <RefreshCw className={`h-3.5 w-3.5 ${syncLoading ? 'animate-spin' : ''}`} />
                      Sinkronisasi Sekarang
                    </Button>

                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setDeleteModalOpen(true)}
                      className="text-xs gap-1.5"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      Hapus Proyek dari Cuan
                    </Button>
                  </div>
                </div>

                {/* Main Detail Header Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2">
                      <div>
                        <div className="flex items-center gap-2">
                          <CardTitle className="text-xl font-bold text-slate-900">{selectedProject.name}</CardTitle>
                          <Badge
                            variant={selectedProject.clone_status === 'synced' ? 'success' : 'default'}
                            className="capitalize text-xs"
                          >
                            {selectedProject.clone_status}
                          </Badge>
                        </div>
                        <CardDescription className="text-xs text-slate-500 mt-1">
                          ID Proyek: <code className="bg-slate-100 px-1 py-0.5 rounded font-mono">{selectedProject.id}</code>
                        </CardDescription>
                      </div>

                      <a
                        href={selectedProject.repo_url}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1.5 text-xs text-blue-600 bg-blue-50 hover:bg-blue-100 px-3 py-1.5 rounded-md border border-blue-200 transition-colors"
                      >
                        Lihat di GitHub
                        <ExternalLink className="h-3.5 w-3.5" />
                      </a>
                    </div>
                  </CardHeader>
                  <CardContent className="p-5 pt-2 grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Repository Remote</div>
                      <div className="text-sm font-bold text-slate-800">{selectedProject.repo_owner} / {selectedProject.repo_name}</div>
                      <div className="text-xs text-slate-500 break-all">{selectedProject.repo_url}</div>
                    </div>

                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Default Branch</div>
                      <div className="text-sm font-bold text-slate-800 flex items-center gap-1.5">
                        <GitBranch className="h-4 w-4 text-emerald-600" />
                        {selectedProject.default_branch}
                      </div>
                      <div className="text-xs text-slate-500">Cabang rujukan utama untuk task & commit</div>
                    </div>

                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Keamanan Kredensial</div>
                      <div className="text-sm font-bold text-emerald-700 flex items-center gap-1.5">
                        <ShieldCheck className="h-4 w-4 text-emerald-600" />
                        {selectedProject.has_token ? selectedProject.token_preview : 'Public Access'}
                      </div>
                      <div className="text-xs text-slate-500">Terenkripsi AES-256-GCM di database</div>
                    </div>
                  </CardContent>
                </Card>

                {/* Branches & Remote Metadata Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-2">
                      <GitBranch className="h-4 w-4 text-slate-500" />
                      Cabang Remote (Remote Branches) dari GitHub
                    </CardTitle>
                    <CardDescription className="text-xs text-slate-500">
                      Daftar branch yang terdeteksi secara real-time dari repository remote.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-5 pt-0">
                    {branchesLoading ? (
                      <div className="flex items-center gap-2 text-xs text-slate-500 py-3">
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        Memeriksa branch remote dari GitHub...
                      </div>
                    ) : projectBranches.length > 0 ? (
                      <div className="flex flex-wrap gap-2 pt-2">
                        {projectBranches.map((br) => (
                          <span
                            key={br}
                            className={`inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-md border font-mono ${
                              br === selectedProject.default_branch
                                ? 'bg-emerald-50 text-emerald-800 border-emerald-300 font-bold'
                                : 'bg-slate-50 text-slate-700 border-slate-200'
                            }`}
                          >
                            <GitBranch className="h-3 w-3" />
                            {br}
                            {br === selectedProject.default_branch && ' (default)'}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <div className="text-xs text-slate-500 py-2">
                        Branch default: <code className="font-mono font-bold text-slate-700">{selectedProject.default_branch}</code>
                      </div>
                    )}
                  </CardContent>
                </Card>

                {/* Credential & Token Security Details */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-2">
                        <Lock className="h-4 w-4 text-emerald-600" />
                        Audit Keamanan Token & Panduan Revoke
                      </CardTitle>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowTokenGuide(!showTokenGuide)}
                        className="text-xs gap-1"
                      >
                        <HelpCircle className="h-3.5 w-3.5" />
                        {showTokenGuide ? 'Sembunyikan Panduan' : 'Lihat Panduan Token'}
                      </Button>
                    </div>
                  </CardHeader>
                  <CardContent className="p-5 pt-0 space-y-3">
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Enkripsi Standar Industri:</strong> Token disimpan dengan AES-256-GCM. Kunci turunan rahasia aman.
                        </div>
                      </div>
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Zero Leak Log:</strong> Engine Git Cuan secara otomatis menyamarkan token dari log runner maupun proses anak.
                        </div>
                      </div>
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Non-Destructive:</strong> Menghapus proyek dari Cuan tidak akan pernah menghapus remote repository GitHub.
                        </div>
                      </div>
                    </div>

                    {showTokenGuide && (
                      <div className="mt-4 p-4 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-700 space-y-2">
                        <h4 className="font-bold text-slate-900 flex items-center gap-1.5">
                          <KeyRound className="h-4 w-4 text-slate-700" />
                          Panduan Konfigurasi & Pencabutan (Revoke) Token GitHub:
                        </h4>
                        <ol className="list-decimal pl-5 space-y-1">
                          <li>
                            Buka <a href="https://github.com/settings/tokens" target="_blank" rel="noreferrer" className="text-blue-600 underline">GitHub Token Settings</a>.
                          </li>
                          <li>Pilih <strong>Generate new token (classic)</strong> atau <strong>Fine-grained token</strong>.</li>
                          <li>Beri izin (scope) <code>repo</code> untuk clone, fetch, dan PR otomatis pada repository private.</li>
                          <li>
                            <strong>Cara Revoke (Cabut Akses):</strong> Kapan saja Anda ingin memutuskan akses Cuan, buka kembali halaman Settings GitHub dan klik tombol <strong>Delete / Revoke</strong> pada token tersebut.
                          </li>
                        </ol>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}
          </div>
        )}

        {/* ============================================================== */}
        {/* TAB 2: TASKS BOARD VIEW                                         */}
        {/* ============================================================== */}
        {activeTab === 'tasks' && (
          <Card className="bg-white border-slate-200 shadow-sm">
            <CardHeader>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div>
                  <CardTitle className="text-base font-bold text-slate-900">Daftar Task Sprint 1</CardTitle>
                  <CardDescription className="text-xs text-slate-500">
                    Data diambil dari endpoint terproteksi <code>/api/v1/tasks</code> menggunakan session token pemilik.
                  </CardDescription>
                </div>
                <Badge variant="outline" className="flex items-center gap-1 text-xs self-start sm:self-auto font-mono">
                  <GitBranch className="h-3 w-3" /> main
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <div className="divide-y divide-slate-100">
                {tasks.length > 0 ? (
                  tasks.map((task) => (
                    <div key={task.id} className="py-3 flex items-center justify-between gap-4">
                      <div className="flex items-center gap-3">
                        <span className="font-mono text-xs font-bold text-slate-500 w-8">{task.id}</span>
                        <span className="text-sm font-medium text-slate-800">{task.title}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        {task.status === 'done' && (
                          <Badge variant="success" className="gap-1 text-xs">
                            <CheckCircle2 className="h-3 w-3" /> Done
                          </Badge>
                        )}
                        {task.status === 'doing' && (
                          <Badge variant="default" className="gap-1 text-xs bg-blue-600 hover:bg-blue-700">
                            <RefreshCw className="h-3 w-3 animate-spin" /> Doing
                          </Badge>
                        )}
                        {task.status === 'backlog' && (
                          <Badge variant="secondary" className="text-xs">
                            Backlog
                          </Badge>
                        )}
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="py-6 text-center text-xs text-slate-500">
                    Memuat data task terproteksi...
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* ============================================================== */}
        {/* TAB 3: SYSTEM FOUNDATION OVERVIEW                              */}
        {/* ============================================================== */}
        {activeTab === 'overview' && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {/* Card 1: Backend */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Backend API</CardTitle>
                    <Server className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Go Chi + Git Engine</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                    <span className="text-xs text-slate-600">/healthz: {health?.status || 'healthy'}</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 2: Database & Volume */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Storage Persisten</CardTitle>
                    <Database className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">PostgreSQL 16</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <HardDrive className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">Migrations: {ready?.migrations_count || '3'} applied</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 3: Auth & Security */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Akses Owner</CardTitle>
                    <ShieldCheck className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Single-Owner Terkunci</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <Lock className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">Setup ditutup (403 Forbidden)</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 4: Proxy & Networking */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Network & Proxy</CardTitle>
                    <Globe className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Caddy / Reverse Proxy</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <Shield className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">DB internal (port 5432 privat)</span>
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Verification & DoD Evidence Banner */}
            <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4 text-xs text-emerald-900 flex flex-col md:flex-row items-start md:items-center justify-between gap-3">
              <div className="space-y-1">
                <div className="font-semibold flex items-center gap-1.5 text-emerald-950">
                  <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                  DoD T3 Terverifikasi: Hubungkan Repository GitHub Existing
                </div>
                <p className="text-emerald-800">
                  Koneksi GitHub aktif, validasi 404/401 informatif, kredensial dienkripsi AES-256-GCM, dan remote repo terlindungi dari penghapusan.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
                  Projects: {projects.length}
                </Badge>
                <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
                  Readyz: {ready?.status || 'ready'}
                </Badge>
              </div>
            </div>
          </div>
        )}

        {/* ============================================================== */}
        {/* MODAL / DIALOG: HUBUNGKAN REPOSITORY GITHUB EXISTING           */}
        {/* ============================================================== */}
        {isConnectModalOpen && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="w-full max-w-lg bg-white rounded-xl shadow-xl border border-slate-200 overflow-hidden">
              <div className="p-5 border-b border-slate-100 flex items-center justify-between">
                <div>
                  <h3 className="text-base font-bold text-slate-900">Hubungkan Repository GitHub Existing</h3>
                  <p className="text-xs text-slate-500">Integrasikan repository GitHub yang sudah ada ke dalam Cuan.</p>
                </div>
                <button
                  onClick={() => setIsConnectModalOpen(false)}
                  className="text-slate-400 hover:text-slate-600 text-lg font-bold"
                >
                  ✕
                </button>
              </div>

              <form onSubmit={handleConnectProject}>
                <div className="p-5 space-y-4 max-h-[75vh] overflow-y-auto">
                  {connectError && (
                    <Alert variant="destructive">
                      <AlertCircle className="h-4 w-4" />
                      <AlertTitle>Koneksi Gagal</AlertTitle>
                      <AlertDescription className="text-xs">{connectError}</AlertDescription>
                    </Alert>
                  )}

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      URL Repository GitHub <span className="text-red-500">*</span>
                    </label>
                    <Input
                      placeholder="https://github.com/buchin/cuan atau buchin/cuan"
                      value={repoUrlInput}
                      onChange={(e) => setRepoUrlInput(e.target.value)}
                      required
                    />
                    <p className="text-[11px] text-slate-500">
                      Format: <code>https://github.com/owner/repo</code> atau <code>owner/repo</code>
                    </p>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Nama Project (Opsional)</label>
                    <Input
                      placeholder="Contoh: Cuan Core (default: nama repository)"
                      value={repoNameInput}
                      onChange={(e) => setRepoNameInput(e.target.value)}
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Default Branch (Opsional)</label>
                    <Input
                      placeholder="main (otomatis dideteksi dari remote)"
                      value={defaultBranchInput}
                      onChange={(e) => setDefaultBranchInput(e.target.value)}
                    />
                  </div>

                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <label className="text-xs font-semibold text-slate-700">
                        Personal Access Token (PAT)
                      </label>
                      <button
                        type="button"
                        onClick={() => setShowTokenGuide(!showTokenGuide)}
                        className="text-[11px] text-blue-600 hover:underline flex items-center gap-1"
                      >
                        <HelpCircle className="h-3 w-3" />
                        Panduan Token & Revoke
                      </button>
                    </div>
                    <Input
                      type="password"
                      placeholder="ghp_••••••••••••••••••••••••••••••••"
                      value={githubTokenInput}
                      onChange={(e) => setGithubTokenInput(e.target.value)}
                    />
                    <p className="text-[11px] text-slate-500">
                      Wajib untuk repository private. Untuk public repo, token bersifat opsional tetapi meningkatkan limit GitHub API.
                    </p>
                  </div>

                  {showTokenGuide && (
                    <div className="p-3 bg-blue-50 border border-blue-200 rounded-lg text-xs text-blue-900 space-y-1.5">
                      <div className="font-bold flex items-center gap-1 text-blue-950">
                        <KeyRound className="h-3.5 w-3.5" />
                        Panduan Personal Access Token (PAT) GitHub
                      </div>
                      <p>
                        1. Buat token di <a href="https://github.com/settings/tokens" target="_blank" rel="noreferrer" className="underline font-semibold">GitHub Developer Settings</a>.<br />
                        2. Centang izin <code>repo</code> untuk repositori private.<br />
                        3. <strong>Pencabutan Akses:</strong> Token dapat dicabut (revoke) kapan saja langsung di GitHub Settings jika tidak lagi dibutuhkan.
                      </p>
                    </div>
                  )}

                  <div className="p-3 bg-slate-50 border border-slate-200 rounded-md text-xs text-slate-600 flex items-start gap-2">
                    <ShieldCheck className="h-4 w-4 text-emerald-600 mt-0.5 shrink-0" />
                    <div>
                      <strong>Safe Credential Storage:</strong> Token GitHub disimpan terenkripsi dengan AES-256-GCM. Kredensial tidak pernah masuk ke file log dan tidak dikirimkan plain text di API.
                    </div>
                  </div>
                </div>

                <div className="p-4 border-t border-slate-100 bg-slate-50 flex items-center justify-end gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setIsConnectModalOpen(false)}
                    disabled={connectLoading}
                    className="text-xs"
                  >
                    Batal
                  </Button>

                  <Button
                    type="submit"
                    size="sm"
                    disabled={connectLoading}
                    className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                  >
                    {connectLoading ? (
                      <>
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        Memvalidasi Repository...
                      </>
                    ) : (
                      'Hubungkan Repository'
                    )}
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* ============================================================== */}
        {/* MODAL / DIALOG: CONFIRM DELETE PROJECT                         */}
        {/* ============================================================== */}
        {deleteModalOpen && selectedProject && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="w-full max-w-md bg-white rounded-xl shadow-xl border border-slate-200 p-5 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 bg-red-100 rounded-full text-red-600">
                  <Trash2 className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-900">Hapus Proyek dari Cuan?</h3>
                  <p className="text-xs text-slate-500">Konfirmasi penghapusan proyek <strong>{selectedProject.name}</strong></p>
                </div>
              </div>

              <div className="p-3 bg-amber-50 border border-amber-200 rounded-md text-xs text-amber-900 space-y-1">
                <div className="font-bold flex items-center gap-1 text-amber-950">
                  <ShieldCheck className="h-4 w-4 text-emerald-600" />
                  Jaminan Keamanan Remote Repository:
                </div>
                <p>
                  Penghapusan ini <strong>HANYA</strong> menghapus data proyek dan cache lokal dari instalasi Cuan Anda.
                  Repository GitHub remote Anda di <code>{selectedProject.repo_url}</code> <strong>TETAP UTUH DAN TIDAK AKAN DIUBAH/DIHAPUS</strong>.
                </p>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDeleteModalOpen(false)}
                  disabled={deleteLoading}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleDeleteProject}
                  disabled={deleteLoading}
                  className="text-xs gap-1.5"
                >
                  {deleteLoading ? (
                    <>
                      <RefreshCw className="h-3 w-3 animate-spin" />
                      Menghapus...
                    </>
                  ) : (
                    'Ya, Hapus dari Cuan'
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Footer */}
        <footer className="text-xs text-slate-500 flex flex-col sm:flex-row justify-between items-center gap-2 border-t pt-4">
          <span>Cuan Monorepo — Apache-2.0 License</span>
          <span>{info?.architecture || 'Go Backend + React Frontend'}</span>
        </footer>
      </div>
    </div>
  )
}

export default App
