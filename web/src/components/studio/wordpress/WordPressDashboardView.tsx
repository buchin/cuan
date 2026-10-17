import { useState, useEffect } from 'react'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import {
  Globe,
  KeyRound,
  ExternalLink,
  RotateCcw,
  RefreshCw,
  AlertCircle,
  Copy,
  Check,
  Eye,
  EyeOff,
  ShieldCheck,
  Rocket,
  Trash2,
  Activity,
  Sparkles,
  StopCircle
} from 'lucide-react'
import {
  WordPressDeploymentDTO,
  WordPressCredentialsDTO,
  WordPressHealthResponse,
  ServerItem
} from '../types'
import { WordPressDeployerModal } from './WordPressDeployerModal'
import { WordPressWebmasterPanel } from './WordPressWebmasterPanel'

export interface WordPressDashboardViewProps {
  projectId: string
  projects?: Array<{ id: string; name: string; slug?: string }>
  baseDomain?: string
  servers?: ServerItem[]
  onOpenDeployerWizard?: (initialStep?: 1 | 2 | 3 | 4) => void
  onToast?: (type: 'success' | 'error', message: string) => void
}

export function WordPressDashboardView({
  projectId: initialProjectId,
  projects = [],
  baseDomain = 'cuan.dojo.cc',
  servers = [],
  onOpenDeployerWizard,
  onToast,
}: WordPressDashboardViewProps) {
  const [selectedProjectId, setSelectedProjectId] = useState<string>(initialProjectId || 'default')
  const [deployment, setDeployment] = useState<WordPressDeploymentDTO | null>(null)
  const [credentials, setCredentials] = useState<WordPressCredentialsDTO | null>(null)
  const [healthData, setHealthData] = useState<WordPressHealthResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [actionInProgress, setActionInProgress] = useState<string | null>(null)

  // Sub-tabs: 'overview' | 'webmaster'
  const [activeTab, setActiveTab] = useState<'overview' | 'webmaster'>('overview')

  // Password masking state
  const [showAdminPassword, setShowAdminPassword] = useState(false)
  const [showDbPassword, setShowDbPassword] = useState(false)
  const [copiedField, setCopiedField] = useState<string | null>(null)

  // Local Modal control
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3 | 4>(1)

  // Keep selected project updated if prop changes
  useEffect(() => {
    if (initialProjectId && initialProjectId !== selectedProjectId) {
      setSelectedProjectId(initialProjectId)
    }
  }, [initialProjectId])

  // Copy helper
  const handleCopy = (text: string, fieldKey: string) => {
    if (!text) return
    navigator.clipboard.writeText(text)
    setCopiedField(fieldKey)
    setTimeout(() => setCopiedField(null), 2500)
  }

  // Fetch all deployment data
  const fetchAllData = async () => {
    if (!selectedProjectId || selectedProjectId === 'default') return
    setLoading(true)
    try {
      // 1. Fetch deployment
      const depRes = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress`)
      if (depRes.ok) {
        const depData: WordPressDeploymentDTO = await depRes.json()
        setDeployment(depData)

        // 2. Fetch credentials
        try {
          const credRes = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress/credentials`)
          if (credRes.ok) {
            const credData: WordPressCredentialsDTO = await credRes.json()
            setCredentials(credData)
          }
        } catch {}

        // 3. Fetch health probe
        try {
          const healthRes = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress/health`)
          if (healthRes.ok) {
            const hData: WordPressHealthResponse = await healthRes.json()
            setHealthData(hData)
          }
        } catch {}
      } else if (depRes.status === 404) {
        setDeployment(null)
        setCredentials(null)
        setHealthData(null)
      }
    } catch (err: any) {
      // Ignore polling errors
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchAllData()
    const timer = setInterval(fetchAllData, 12000)
    return () => clearInterval(timer)
  }, [selectedProjectId])

  // Restart WordPress runtime
  const handleRestart = async () => {
    setActionInProgress('restart')
    try {
      const res = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress/restart`, {
        method: 'POST',
      })
      if (res.ok) {
        onToast?.('success', '⚡ WordPress instance berhasil di-restart.')
        fetchAllData()
      } else {
        const err = await res.json().catch(() => ({}))
        onToast?.('error', err.error || 'Gagal me-restart WordPress.')
      }
    } catch (err: any) {
      onToast?.('error', err.message || 'Gagal me-restart WordPress.')
    } finally {
      setActionInProgress(null)
    }
  }

  // Stop WordPress runtime
  const handleStop = async () => {
    if (!confirm('Apakah Anda yakin ingin menghentikan sementara WordPress container?')) return
    setActionInProgress('stop')
    try {
      const res = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress/stop`, {
        method: 'POST',
      })
      if (res.ok) {
        onToast?.('success', 'WordPress container telah dihentikan.')
        fetchAllData()
      } else {
        const err = await res.json().catch(() => ({}))
        onToast?.('error', err.error || 'Gagal menghentikan WordPress.')
      }
    } catch (err: any) {
      onToast?.('error', err.message || 'Gagal menghentikan WordPress.')
    } finally {
      setActionInProgress(null)
    }
  }

  // Delete WordPress deployment
  const handleDelete = async () => {
    if (!confirm('PERINGATAN: Seluruh instalasi WordPress dan database MySQL akan dihapus permanen. Lanjutkan?')) return
    setActionInProgress('delete')
    try {
      const res = await fetch(`/api/v1/projects/${selectedProjectId}/wordpress`, {
        method: 'DELETE',
      })
      if (res.ok) {
        onToast?.('success', 'Instalasi WordPress berhasil dihapus.')
        setDeployment(null)
        setCredentials(null)
      } else {
        const err = await res.json().catch(() => ({}))
        onToast?.('error', err.error || 'Gagal menghapus WordPress.')
      }
    } catch (err: any) {
      onToast?.('error', err.message || 'Gagal menghapus WordPress.')
    } finally {
      setActionInProgress(null)
    }
  }

  // Trigger launchpad wizard
  const handleOpenLaunchpad = (step: 1 | 2 | 3 | 4 = 1) => {
    if (onOpenDeployerWizard) {
      onOpenDeployerWizard(step)
    } else {
      setWizardStep(step)
      setIsModalOpen(true)
    }
  }

  // Current project metadata
  const currentProject = projects.find((p) => p.id === selectedProjectId)
  const projectName = currentProject?.name || selectedProjectId
  const projectSlug = currentProject?.slug || ''

  // Derived statuses
  const isRunning = deployment?.status === 'running'
  const isHealthy = deployment?.health_status === 'healthy'
  const isDegraded = isRunning && deployment?.health_status !== 'healthy'
  const isStopped = deployment?.status === 'stopped'

  const siteUrl = deployment?.domain
    ? `https://${deployment.domain}`
    : deployment?.port
    ? `http://127.0.0.1:${deployment.port}`
    : ''
  const adminUrl = siteUrl ? `${siteUrl}/wp-login.php` : ''

  return (
    <div className="space-y-6">
      {/* Top Header & Project Selector */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xs">
        <div className="flex items-center gap-3.5">
          <div className="p-3 rounded-2xl bg-blue-500/10 dark:bg-blue-500/20 text-blue-600 dark:text-blue-400 border border-blue-500/20 shadow-xs">
            <Globe className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-bold text-slate-900 dark:text-white">
                Turnkey WordPress Cloud
              </h1>
              <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20 text-[11px]">
                Sprint 13
              </Badge>
            </div>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              Kelola instalasi WordPress mandiri, brankas AES-256-GCM, dan AI Webmaster Engine.
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2.5 w-full sm:w-auto justify-between sm:justify-end">
          {/* Project Switcher */}
          {projects.length > 0 && (
            <select
              value={selectedProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="text-xs px-3 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl text-slate-800 dark:text-slate-200 font-medium"
            >
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={fetchAllData}
            disabled={loading}
            className="h-8 text-xs text-slate-600 dark:text-slate-300"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Segarkan
          </Button>

          <Button
            size="sm"
            onClick={() => handleOpenLaunchpad(1)}
            className="h-8 text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium shadow-xs"
          >
            <Rocket className="h-3.5 w-3.5 mr-1" />
            <span>{deployment ? 'Re-Deploy / Launchpad' : '1-Click Launchpad'}</span>
          </Button>
        </div>
      </div>

      {/* Main Content Area */}
      {deployment ? (
        <div className="space-y-6">
          {/* Active Deployment Summary Hero Bar */}
          <div className="p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xs space-y-4">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-4 border-b border-slate-100 dark:border-slate-800">
              <div className="space-y-1">
                <div className="flex items-center gap-2.5 flex-wrap">
                  <h2 className="text-base font-bold text-slate-900 dark:text-white">
                    {deployment.site_title || 'WordPress Site'}
                  </h2>

                  {isHealthy ? (
                    <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 flex items-center gap-1.5 text-xs font-semibold px-2 py-0.5">
                      <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                      Running (Healthy)
                    </Badge>
                  ) : isDegraded ? (
                    <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 flex items-center gap-1.5 text-xs font-semibold px-2 py-0.5">
                      <AlertCircle className="h-3.5 w-3.5" />
                      Degraded
                    </Badge>
                  ) : isStopped ? (
                    <Badge className="bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20 text-xs px-2 py-0.5 font-semibold">
                      Stopped
                    </Badge>
                  ) : (
                    <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20 text-xs px-2 py-0.5 font-semibold">
                      {deployment.status}
                    </Badge>
                  )}

                  <Badge className="bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border-none font-mono text-[11px]">
                    Port: {deployment.port}
                  </Badge>

                  <Badge className="bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border-none font-mono text-[11px]">
                    {deployment.wp_version || 'latest'}
                  </Badge>
                </div>

                <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                  <span>Domain:</span>
                  <a
                    href={siteUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="font-mono text-blue-600 dark:text-blue-400 hover:underline inline-flex items-center gap-1"
                  >
                    <span>{deployment.domain}</span>
                    <ExternalLink className="h-3 w-3" />
                  </a>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex flex-wrap items-center gap-2">
                <a
                  href={siteUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-medium text-xs shadow-xs"
                >
                  <Globe className="h-3.5 w-3.5" />
                  <span>Visit Site</span>
                  <ExternalLink className="h-3 w-3" />
                </a>

                <a
                  href={adminUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs shadow-xs"
                >
                  <KeyRound className="h-3.5 w-3.5" />
                  <span>Open WP-Admin</span>
                  <ExternalLink className="h-3 w-3" />
                </a>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleRestart}
                  disabled={actionInProgress === 'restart'}
                  className="h-8 text-xs text-slate-700 dark:text-slate-300"
                  title="Restart runtime container"
                >
                  <RotateCcw className={`h-3.5 w-3.5 mr-1 ${actionInProgress === 'restart' ? 'animate-spin text-blue-500' : ''}`} />
                  Restart
                </Button>

                {isRunning && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleStop}
                    disabled={actionInProgress === 'stop'}
                    className="h-8 text-xs text-slate-700 dark:text-slate-300"
                    title="Stop runtime container"
                  >
                    <StopCircle className="h-3.5 w-3.5 mr-1 text-amber-500" />
                    Stop
                  </Button>
                )}

                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleDelete}
                  disabled={actionInProgress === 'delete'}
                  className="h-8 text-xs text-rose-500 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/20"
                  title="Delete deployment"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>

            {/* Sub-tab Navigation */}
            <div className="flex items-center gap-2 border-b border-slate-100 dark:border-slate-800 pb-2">
              <button
                onClick={() => setActiveTab('overview')}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all ${
                  activeTab === 'overview'
                    ? 'bg-blue-600 text-white shadow-xs'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                <Activity className="h-4 w-4" />
                <span>Overview & Credentials Vault</span>
              </button>

              <button
                onClick={() => setActiveTab('webmaster')}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all ${
                  activeTab === 'webmaster'
                    ? 'bg-blue-600 text-white shadow-xs'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                <Sparkles className="h-4 w-4" />
                <span>AI Webmaster Studio (Dropzone, Scripts & Config)</span>
              </button>
            </div>

            {/* TAB CONTENT: Overview */}
            {activeTab === 'overview' && (
              <div className="space-y-6 pt-2">
                {/* Credentials Vault Card */}
                <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/60 space-y-3">
                  <div className="flex items-center justify-between border-b border-slate-200/60 dark:border-slate-700/60 pb-2.5">
                    <div className="flex items-center gap-2">
                      <ShieldCheck className="h-4 w-4 text-blue-500" />
                      <h3 className="text-xs font-bold text-slate-900 dark:text-white">
                        Admin & Database Credentials Vault (AES-256-GCM)
                      </h3>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          const all = `[WordPress Turnkey Credentials]\nSite: ${siteUrl}\nAdmin URL: ${adminUrl}\nUsername: ${deployment.admin_username}\nPassword: ${credentials?.admin_password || ''}\nDatabase: ${deployment.db_name}\nDB User: ${deployment.db_user}\nDB Host: ${deployment.db_host}:${deployment.db_port}`
                          handleCopy(all, 'all_creds')
                        }}
                        className="h-6 text-[11px] text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        {copiedField === 'all_creds' ? (
                          <>
                            <Check className="h-3 w-3 mr-1 text-emerald-500" />
                            <span>Copied All!</span>
                          </>
                        ) : (
                          <>
                            <Copy className="h-3 w-3 mr-1" />
                            <span>Copy All Credentials</span>
                          </>
                        )}
                      </Button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 text-xs">
                    {/* Admin Username */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">Admin Username</span>
                      <div className="flex items-center justify-between font-mono font-semibold text-slate-800 dark:text-slate-200">
                        <span>{deployment.admin_username}</span>
                        <button
                          onClick={() => handleCopy(deployment.admin_username, 'admin_user')}
                          className="text-slate-400 hover:text-slate-600"
                          title="Copy username"
                        >
                          {copiedField === 'admin_user' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                        </button>
                      </div>
                    </div>

                    {/* Admin Password with Mask Toggle */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">Admin Password</span>
                      <div className="flex items-center justify-between font-mono font-semibold text-slate-800 dark:text-slate-200">
                        <span>
                          {showAdminPassword
                            ? credentials?.admin_password || '(Tersimpan Terenkripsi)'
                            : '••••••••••••••••••••••••'}
                        </span>
                        <div className="flex items-center gap-1.5 ml-2">
                          <button
                            onClick={() => setShowAdminPassword(!showAdminPassword)}
                            className="text-slate-400 hover:text-slate-600"
                            title={showAdminPassword ? 'Sembunyikan password' : 'Lihat password'}
                          >
                            {showAdminPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                          </button>
                          <button
                            onClick={() => handleCopy(credentials?.admin_password || '', 'admin_pass')}
                            className="text-slate-400 hover:text-slate-600"
                            title="Copy password"
                          >
                            {copiedField === 'admin_pass' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                          </button>
                        </div>
                      </div>
                    </div>

                    {/* Admin Email */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">Admin Email</span>
                      <div className="flex items-center justify-between font-mono text-slate-800 dark:text-slate-200">
                        <span className="truncate">{deployment.admin_email}</span>
                        <button
                          onClick={() => handleCopy(deployment.admin_email, 'admin_email')}
                          className="text-slate-400 hover:text-slate-600 ml-1"
                        >
                          {copiedField === 'admin_email' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                        </button>
                      </div>
                    </div>

                    {/* DB Name & Host */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">Database Schema</span>
                      <div className="flex items-center justify-between font-mono text-slate-800 dark:text-slate-200">
                        <span>{deployment.db_name}</span>
                        <Badge className="text-[10px] bg-slate-100 dark:bg-slate-800">
                          {deployment.db_host}:{deployment.db_port}
                        </Badge>
                      </div>
                    </div>

                    {/* DB User */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">DB Username</span>
                      <div className="flex items-center justify-between font-mono text-slate-800 dark:text-slate-200">
                        <span>{deployment.db_user}</span>
                        <button
                          onClick={() => handleCopy(deployment.db_user, 'db_user')}
                          className="text-slate-400 hover:text-slate-600"
                        >
                          {copiedField === 'db_user' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                        </button>
                      </div>
                    </div>

                    {/* DB Password */}
                    <div className="p-3 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                      <span className="text-[11px] text-slate-400 font-medium">DB Password</span>
                      <div className="flex items-center justify-between font-mono text-slate-800 dark:text-slate-200">
                        <span>
                          {showDbPassword
                            ? credentials?.db_password || '(Tersimpan Terenkripsi)'
                            : '••••••••••••••••••••••••'}
                        </span>
                        <div className="flex items-center gap-1.5 ml-2">
                          <button
                            onClick={() => setShowDbPassword(!showDbPassword)}
                            className="text-slate-400 hover:text-slate-600"
                          >
                            {showDbPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                          </button>
                          <button
                            onClick={() => handleCopy(credentials?.db_password || '', 'db_pass')}
                            className="text-slate-400 hover:text-slate-600"
                          >
                            {copiedField === 'db_pass' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Metrics Breakdown Grid */}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                  <div className="p-4 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                    <span className="text-xs text-slate-400 font-medium">HTTP Probe Health</span>
                    <div className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-1.5">
                      <span className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                      <span>{healthData?.health_status || deployment.health_status || 'healthy'}</span>
                    </div>
                    <p className="text-[11px] text-slate-400">
                      Response: {healthData?.probe?.response_time_ms ? `${healthData.probe.response_time_ms}ms` : '< 50ms'}
                    </p>
                  </div>

                  <div className="p-4 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                    <span className="text-xs text-slate-400 font-medium">Permalink SEO</span>
                    <div className="text-sm font-bold font-mono text-slate-900 dark:text-white truncate">
                      {deployment.permalink_structure || '/%postname%/'}
                    </div>
                    <p className="text-[11px] text-slate-400">SEO-friendly rewrite rules</p>
                  </div>

                  <div className="p-4 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                    <span className="text-xs text-slate-400 font-medium">Table Prefix</span>
                    <div className="text-sm font-bold font-mono text-slate-900 dark:text-white">
                      {deployment.table_prefix || 'wp_'}
                    </div>
                    <p className="text-[11px] text-slate-400">Isolated database namespace</p>
                  </div>

                  <div className="p-4 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                    <span className="text-xs text-slate-400 font-medium">Reverse Proxy</span>
                    <div className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-1">
                      <span>Caddy Auto-TLS</span>
                    </div>
                    <p className="text-[11px] text-slate-400">Automatic Let's Encrypt / ZeroSSL</p>
                  </div>
                </div>
              </div>
            )}

            {/* TAB CONTENT: AI Webmaster Studio */}
            {activeTab === 'webmaster' && (
              <div className="pt-2">
                <WordPressWebmasterPanel
                  projectId={selectedProjectId}
                  projectName={projectName}
                  deploymentId={deployment.id}
                  onToast={onToast}
                />
              </div>
            )}
          </div>
        </div>
      ) : (
        /* Empty State: No WordPress Deployment Found */
        <div className="p-12 text-center rounded-3xl bg-white dark:bg-slate-900 border border-dashed border-slate-200 dark:border-slate-800 space-y-6 max-w-2xl mx-auto shadow-sm">
          <div className="w-16 h-16 rounded-3xl bg-blue-500/10 dark:bg-blue-500/20 text-blue-600 dark:text-blue-400 flex items-center justify-center mx-auto shadow-inner">
            <Globe className="h-8 w-8" />
          </div>

          <div className="space-y-2">
            <h2 className="text-lg font-bold text-slate-900 dark:text-white">
              Belum Ada Instalasi WordPress di Proyek Ini
            </h2>
            <p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
              Luncurkan website WordPress dalam &lt; 60 detik dengan automated MySQL container, WP-CLI setup, SSL HTTPS otomatis, brankas kredensial AES-256-GCM, dan AI Webmaster Engine.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-left pt-2">
            <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200/60 dark:border-slate-700/60 text-xs">
              <span className="font-semibold text-slate-800 dark:text-slate-200 block mb-1">
                🚀 1-Click Launchpad
              </span>
              <p className="text-[11px] text-slate-400">
                Setup core, database MySQL, dan permalink SEO otomatis tanpa layar setup web manual.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200/60 dark:border-slate-700/60 text-xs">
              <span className="font-semibold text-slate-800 dark:text-slate-200 block mb-1">
                🔒 AES-256-GCM Vault
              </span>
              <p className="text-[11px] text-slate-400">
                Kredensial admin dan DB terenkripsi dengan proteksi zero plaintext leakage pada API.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200/60 dark:border-slate-700/60 text-xs">
              <span className="font-semibold text-slate-800 dark:text-slate-200 block mb-1">
                🤖 AI Webmaster
              </span>
              <p className="text-[11px] text-slate-400">
                Injeksi script tracking aman via mu-plugins dan dropzone ZIP theme/plugin kustom.
              </p>
            </div>
          </div>

          <div className="pt-2">
            <Button
              onClick={() => handleOpenLaunchpad(1)}
              className="bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs px-6 py-2.5 rounded-xl shadow-md shadow-blue-500/20"
            >
              <Rocket className="h-4 w-4 mr-1.5" />
              Mulai 1-Click WordPress Deployer
            </Button>
          </div>
        </div>
      )}

      {/* Internal Deployer Modal Fallback */}
      <WordPressDeployerModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        projectId={selectedProjectId}
        projectName={projectName}
        projectSlug={projectSlug}
        baseDomain={baseDomain}
        servers={servers}
        initialStep={wizardStep}
        onDeploymentSuccess={(newDep) => {
          setDeployment(newDep)
          fetchAllData()
        }}
      />
    </div>
  )
}
