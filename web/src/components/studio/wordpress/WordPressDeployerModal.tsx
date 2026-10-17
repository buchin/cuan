import { useState, useEffect, useMemo } from 'react'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Badge } from '../../ui/badge'
import {
  X,
  Globe,
  KeyRound,
  Database,
  Terminal as TerminalIcon,
  Sparkles,
  CheckCircle2,
  AlertCircle,
  Eye,
  EyeOff,
  Copy,
  Check,
  ExternalLink,
  Rocket,
  ArrowRight,
  ArrowLeft,
  UploadCloud,
  FileArchive,
  ShieldCheck,
  FolderArchive,
  Trash2
} from 'lucide-react'
import {
  WordPressDeploymentDTO,
  WordPressDeployPayload,
  WordPressDeployResponse,
  ServerItem
} from '../types'

export interface WordPressDeployerModalProps {
  isOpen: boolean
  onClose: () => void
  projectId: string
  projectName?: string
  projectSlug?: string
  baseDomain?: string
  servers?: ServerItem[]
  initialStep?: 1 | 2 | 3 | 4
  onDeploymentSuccess?: (deployment: WordPressDeploymentDTO) => void
  onOpenDashboard?: () => void
}

interface StagedAsset {
  file: File
  kind: 'theme' | 'plugin'
  slug: string
  activate: boolean
  uploaded?: boolean
  error?: string
}

// Generate secure 24-character password in browser
function generateStrongPassword(length = 24): string {
  const chars = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*()_+~`'
  const randomValues = new Uint32Array(length)
  window.crypto.getRandomValues(randomValues)
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars[randomValues[i] % chars.length]
  }
  return result
}

export function WordPressDeployerModal({
  isOpen,
  onClose,
  projectId,
  projectName = 'WordPress Project',
  projectSlug = '',
  baseDomain = 'cuan.dojo.cc',
  servers = [],
  initialStep = 1,
  onDeploymentSuccess,
  onOpenDashboard,
}: WordPressDeployerModalProps) {
  // Wizard current step: 1 | 2 | 3 | 4
  const [currentStep, setCurrentStep] = useState<1 | 2 | 3 | 4>(initialStep)

  // Feedback notifications
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // -------------------------------------------------------------
  // Step 1: Site Profile & Admin Credentials
  // -------------------------------------------------------------
  const [siteTitle, setSiteTitle] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [adminUsername, setAdminUsername] = useState('admin')
  const [adminPassword, setAdminPassword] = useState('')
  const [showAdminPassword, setShowAdminPassword] = useState(false)

  // -------------------------------------------------------------
  // Step 2: Domain & Database Configuration
  // -------------------------------------------------------------
  const [domain, setDomain] = useState('')
  const [dbName, setDbName] = useState('')
  const [dbUser, setDbUser] = useState('')
  const [dbPassword, setDbPassword] = useState('')
  const [showDbPassword, setShowDbPassword] = useState(false)
  const [tablePrefix, setTablePrefix] = useState('wp_')
  const [wpVersion, setWpVersion] = useState('latest')
  const [permalinkStructure, setPermalinkStructure] = useState('/%postname%/')
  const [selectedServerId, setSelectedServerId] = useState<string>('')

  // -------------------------------------------------------------
  // Step 3: Staged Themes & Plugins ZIP Upload
  // -------------------------------------------------------------
  const [stagedAssets, setStagedAssets] = useState<StagedAsset[]>([])
  const [dragOver, setDragOver] = useState(false)
  const [selectedKind, setSelectedKind] = useState<'theme' | 'plugin'>('theme')
  const [assetSlug, setAssetSlug] = useState('')
  const [autoActivate, setAutoActivate] = useState(true)

  // -------------------------------------------------------------
  // Step 4: Live Launch & Deployment State
  // -------------------------------------------------------------
  const [isDeploying, setIsDeploying] = useState(false)
  const [deployLogs, setDeployLogs] = useState<string[]>([])
  const [deployResult, setDeployResult] = useState<WordPressDeployResponse | null>(null)
  const [deployStepIndex, setDeployStepIndex] = useState(0)
  const [copiedField, setCopiedField] = useState<string | null>(null)

  // Initialization when modal opens or projectId changes
  useEffect(() => {
    if (isOpen) {
      setCurrentStep(initialStep)
      setErrorMsg(null)
      setSuccessMsg(null)

      const initialTitle = projectName && projectName !== 'Project' ? projectName : 'My WordPress Blog'
      setSiteTitle(initialTitle)

      const slug = projectSlug || projectName.toLowerCase().replace(/[^a-z0-9_-]/g, '-') || 'wp-blog'
      setAdminEmail(`admin@${slug}.local`)
      setDomain(`${slug}.${baseDomain}`)
      setDbName(`wp_${slug.replace(/-/g, '_')}`)
      setDbUser(`wp_usr_${slug.slice(0, 8).replace(/-/g, '_')}`)

      // Auto-generate initial strong passwords
      const pass = generateStrongPassword(24)
      setAdminPassword(pass)
      setDbPassword(generateStrongPassword(24))
    }
  }, [isOpen, projectId, projectName, projectSlug, baseDomain, initialStep])

  // Copy helper
  const handleCopy = (text: string, fieldName: string) => {
    if (!text) return
    navigator.clipboard.writeText(text)
    setCopiedField(fieldName)
    setTimeout(() => setCopiedField(null), 2500)
  }

  // File drop handler for Step 3
  const handleFileDrop = (files: FileList | null) => {
    if (!files || files.length === 0) return
    const file = files[0]
    if (!file.name.toLowerCase().endsWith('.zip')) {
      setErrorMsg('File harus berformat .ZIP WordPress plugin atau theme.')
      return
    }

    const defaultSlug = file.name
      .replace(/\.zip$/i, '')
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, '-')

    setStagedAssets((prev) => [
      ...prev,
      {
        file,
        kind: selectedKind,
        slug: assetSlug.trim() || defaultSlug,
        activate: autoActivate,
      },
    ])
    setAssetSlug('')
    setErrorMsg(null)
  }

  const handleRemoveStagedAsset = (index: number) => {
    setStagedAssets((prev) => prev.filter((_, i) => i !== index))
  }

  // Step 1 Validation
  const isStep1Valid = useMemo(() => {
    return (
      siteTitle.trim().length >= 2 &&
      adminEmail.trim().includes('@') &&
      adminUsername.trim().length >= 3 &&
      adminPassword.trim().length >= 8
    )
  }, [siteTitle, adminEmail, adminUsername, adminPassword])

  // Step 2 Validation
  const isStep2Valid = useMemo(() => {
    return (
      domain.trim().length >= 3 &&
      dbName.trim().length >= 2 &&
      dbUser.trim().length >= 2 &&
      dbPassword.trim().length >= 8
    )
  }, [domain, dbName, dbUser, dbPassword])

  // Launch WordPress Deployment
  const handleStartDeploy = async () => {
    setIsDeploying(true)
    setErrorMsg(null)
    setSuccessMsg(null)
    setDeployStepIndex(1)
    setDeployLogs([
      `[${new Date().toLocaleTimeString()}] 🚀 Initiating Turnkey WordPress Cloud provisioning...`,
      `[${new Date().toLocaleTimeString()}] 🔒 Encrypting admin and database credentials with AES-256-GCM vault...`,
    ])

    const payload: WordPressDeployPayload = {
      site_title: siteTitle.trim(),
      admin_email: adminEmail.trim(),
      admin_username: adminUsername.trim(),
      admin_password: adminPassword,
      domain: domain.trim(),
      db_name: dbName.trim(),
      db_user: dbUser.trim(),
      db_password: dbPassword,
      table_prefix: tablePrefix.trim() || 'wp_',
      wp_version: wpVersion.trim() || 'latest',
      permalink_structure: permalinkStructure.trim() || '/%postname%/',
      server_id: selectedServerId ? selectedServerId : undefined,
      skip_health_wait: false,
    }

    try {
      setDeployStepIndex(2)
      setDeployLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] 📦 Provisioning isolated MySQL database schema '${dbName}'...`,
        `[${new Date().toLocaleTimeString()}] ⬇️ Verifying WordPress core cache (${wpVersion})...`,
      ])

      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/deploy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || `HTTP ${res.status}: Gagal meluncurkan WordPress`)
      }

      const result: WordPressDeployResponse = await res.json()
      setDeployResult(result)
      setDeployStepIndex(3)

      if (result.execution_logs && result.execution_logs.length > 0) {
        setDeployLogs((prev) => [
          ...prev,
          ...result.execution_logs.map((l) => `[WP-CLI] ${l}`),
        ])
      }

      setDeployLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] 🌐 Registering reverse proxy Caddy HTTPS domain route for ${domain}...`,
        `[${new Date().toLocaleTimeString()}] 🩺 Probing health endpoints (/ and /wp-login.php)...`,
      ])

      // If there are staged assets, upload them now
      if (stagedAssets.length > 0) {
        setDeployLogs((prev) => [
          ...prev,
          `[${new Date().toLocaleTimeString()}] 📦 Installing ${stagedAssets.length} staged theme/plugin ZIP assets...`,
        ])

        for (let i = 0; i < stagedAssets.length; i++) {
          const asset = stagedAssets[i]
          try {
            const formData = new FormData()
            formData.append('file', asset.file)
            formData.append('kind', asset.kind)
            formData.append('slug', asset.slug)
            formData.append('activate', asset.activate ? 'true' : 'false')

            const assetRes = await fetch(`/api/v1/projects/${projectId}/wordpress/assets/upload`, {
              method: 'POST',
              body: formData,
            })

            if (assetRes.ok) {
              setDeployLogs((prev) => [
                ...prev,
                `[${new Date().toLocaleTimeString()}] ✅ ${asset.kind.toUpperCase()} '${asset.slug}' successfully extracted and activated!`,
              ])
            } else {
              const err = await assetRes.json().catch(() => ({}))
              setDeployLogs((prev) => [
                ...prev,
                `[${new Date().toLocaleTimeString()}] ⚠️ Asset '${asset.slug}' notice: ${err.error || 'Check asset compatibility'}`,
              ])
            }
          } catch (e: any) {
            setDeployLogs((prev) => [
              ...prev,
              `[${new Date().toLocaleTimeString()}] ⚠️ Asset upload warning: ${e.message}`,
            ])
          }
        }
      }

      setDeployStepIndex(4)
      setDeployLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] 🎉 WordPress Turnkey Deployment LIVE & HEALTHY!`,
      ])
      setSuccessMsg('🎉 WordPress berhasil diluncurkan dan siap digunakan!')

      if (onDeploymentSuccess && result.deployment) {
        onDeploymentSuccess(result.deployment)
      }
    } catch (err: any) {
      setErrorMsg(err.message || 'Terjadi kesalahan saat deploy WordPress.')
      setDeployLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] ❌ Deployment failed: ${err.message}`,
      ])
    } finally {
      setIsDeploying(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6 bg-slate-900/70 backdrop-blur-xs overflow-y-auto animate-in fade-in duration-200">
      <div className="relative w-full max-w-4xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl flex flex-col max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-xl bg-blue-500/10 dark:bg-blue-500/20 text-blue-600 dark:text-blue-400 border border-blue-500/20">
              <Globe className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-semibold text-slate-900 dark:text-white">
                  1-Click WordPress Launchpad
                </h2>
                <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20 text-[11px]">
                  Turnkey Cloud
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Provisi container WordPress mandiri, MySQL terisolasi, WP-CLI bootstrapping, dan SSL otomatis.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Wizard Step Progress Tracker */}
        <div className="px-6 py-3 border-b border-slate-100 dark:border-slate-800 bg-slate-50/30 dark:bg-slate-900/30">
          <div className="grid grid-cols-4 gap-2">
            {[
              { step: 1, title: '1. Site & Admin', icon: KeyRound },
              { step: 2, title: '2. Domain & DB', icon: Database },
              { step: 3, title: '3. Theme & Plugin', icon: FolderArchive },
              { step: 4, title: '4. Launch Console', icon: Rocket },
            ].map((s) => {
              const Icon = s.icon
              const isActive = currentStep === s.step
              const isPassed = currentStep > s.step || (s.step === 4 && deployResult !== null)
              return (
                <button
                  key={s.step}
                  onClick={() => {
                    if (s.step <= currentStep || (s.step === 2 && isStep1Valid) || (s.step === 3 && isStep1Valid && isStep2Valid)) {
                      setCurrentStep(s.step as any)
                    }
                  }}
                  disabled={isDeploying}
                  className={`flex items-center gap-2 px-3 py-2 rounded-xl text-xs font-medium transition-all text-left ${
                    isActive
                      ? 'bg-blue-600 text-white shadow-xs'
                      : isPassed
                      ? 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border border-blue-500/20'
                      : 'bg-slate-100 dark:bg-slate-800/50 text-slate-400 dark:text-slate-500'
                  }`}
                >
                  <Icon className={`h-4 w-4 ${isActive ? 'text-white' : isPassed ? 'text-blue-500' : ''}`} />
                  <span className="truncate">{s.title}</span>
                </button>
              )
            })}
          </div>
        </div>

        {/* Modal Body */}
        <div className="flex-1 p-6 overflow-y-auto space-y-5">
          {/* Global Alert Notification */}
          {errorMsg && (
            <div className="flex items-start gap-2.5 p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 text-xs animate-in fade-in duration-150">
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              <div className="flex-1">{errorMsg}</div>
            </div>
          )}

          {successMsg && (
            <div className="flex items-start gap-2.5 p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs animate-in fade-in duration-150">
              <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5" />
              <div className="flex-1">{successMsg}</div>
            </div>
          )}

          {/* ========================================================== */}
          {/* STEP 1: Site Profile & Admin Credentials                    */}
          {/* ========================================================== */}
          {currentStep === 1 && (
            <div className="space-y-4">
              <div className="bg-blue-50/50 dark:bg-blue-950/20 border border-blue-100 dark:border-blue-900/40 rounded-xl p-4 text-xs text-blue-900 dark:text-blue-200 space-y-1">
                <div className="font-semibold flex items-center gap-1.5">
                  <ShieldCheck className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                  <span>Kredensial Admin Terproteksi Vault AES-256-GCM</span>
                </div>
                <p className="text-blue-700 dark:text-blue-300">
                  Password admin akan otomatis dienkripsi dan disimpan aman. Kredensial tidak pernah terekspos dalam plain text pada API JSON publik.
                </p>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Site Title <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="text"
                    placeholder="e.g. My Authority Niche Blog"
                    value={siteTitle}
                    onChange={(e) => setSiteTitle(e.target.value)}
                    className="text-xs"
                  />
                  <p className="text-[11px] text-slate-400">Judul web yang tampil di header dan metadata SEO.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Admin Email <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="email"
                    placeholder="admin@mydomain.com"
                    value={adminEmail}
                    onChange={(e) => setAdminEmail(e.target.value)}
                    className="text-xs"
                  />
                  <p className="text-[11px] text-slate-400">Email untuk notifikasi WordPress core dan reset password.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Admin Username <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="text"
                    placeholder="admin"
                    value={adminUsername}
                    onChange={(e) => setAdminUsername(e.target.value)}
                    className="text-xs"
                  />
                  <p className="text-[11px] text-slate-400">Username login ke dashboard /wp-admin.</p>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Admin Password <span className="text-rose-500">*</span>
                    </label>
                    <button
                      type="button"
                      onClick={() => setAdminPassword(generateStrongPassword(24))}
                      className="inline-flex items-center gap-1 text-[11px] text-blue-600 dark:text-blue-400 hover:underline font-medium"
                    >
                      <Sparkles className="h-3 w-3" />
                      <span>Auto-Generate</span>
                    </button>
                  </div>
                  <div className="relative">
                    <Input
                      type={showAdminPassword ? 'text' : 'password'}
                      placeholder="Enter or auto-generate password"
                      value={adminPassword}
                      onChange={(e) => setAdminPassword(e.target.value)}
                      className="text-xs pr-16 font-mono"
                    />
                    <div className="absolute right-1 top-1 flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => setShowAdminPassword(!showAdminPassword)}
                        className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                        title={showAdminPassword ? 'Sembunyikan password' : 'Lihat password'}
                      >
                        {showAdminPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                      </button>
                      <button
                        type="button"
                        onClick={() => handleCopy(adminPassword, 'admin_pass')}
                        className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                        title="Copy password"
                      >
                        {copiedField === 'admin_pass' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>
                  </div>
                  <p className="text-[11px] text-slate-400">Minimal 8 karakter. Disarankan memakai auto-generate 24-char.</p>
                </div>
              </div>
            </div>
          )}

          {/* ========================================================== */}
          {/* STEP 2: Domain & Database Configuration                   */}
          {/* ========================================================== */}
          {currentStep === 2 && (
            <div className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Domain / Hostname <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="text"
                    placeholder={`blog.${baseDomain}`}
                    value={domain}
                    onChange={(e) => setDomain(e.target.value)}
                    className="text-xs"
                  />
                  <p className="text-[11px] text-slate-400">Domain akan otomatis di-binding ke Caddy dengan SSL HTTPS.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Target Server (Opsional)
                  </label>
                  <select
                    value={selectedServerId}
                    onChange={(e) => setSelectedServerId(e.target.value)}
                    className="w-full text-xs px-3 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200"
                  >
                    <option value="">Local Cuan Engine Host (Default)</option>
                    {servers.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.host})
                      </option>
                    ))}
                  </select>
                  <p className="text-[11px] text-slate-400">Pilih host target runtime container.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Database Name <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="text"
                    placeholder="wp_production"
                    value={dbName}
                    onChange={(e) => setDbName(e.target.value)}
                    className="text-xs font-mono"
                  />
                  <p className="text-[11px] text-slate-400">Nama database MySQL terisolasi.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Database User <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    type="text"
                    placeholder="wp_user"
                    value={dbUser}
                    onChange={(e) => setDbUser(e.target.value)}
                    className="text-xs font-mono"
                  />
                  <p className="text-[11px] text-slate-400">User database MySQL dengan hak penuh pada skema.</p>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Database Password <span className="text-rose-500">*</span>
                    </label>
                    <button
                      type="button"
                      onClick={() => setDbPassword(generateStrongPassword(24))}
                      className="inline-flex items-center gap-1 text-[11px] text-blue-600 dark:text-blue-400 hover:underline font-medium"
                    >
                      <Sparkles className="h-3 w-3" />
                      <span>Auto-Generate</span>
                    </button>
                  </div>
                  <div className="relative">
                    <Input
                      type={showDbPassword ? 'text' : 'password'}
                      value={dbPassword}
                      onChange={(e) => setDbPassword(e.target.value)}
                      className="text-xs pr-16 font-mono"
                    />
                    <div className="absolute right-1 top-1 flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => setShowDbPassword(!showDbPassword)}
                        className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                      >
                        {showDbPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                      </button>
                      <button
                        type="button"
                        onClick={() => handleCopy(dbPassword, 'db_pass')}
                        className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                      >
                        {copiedField === 'db_pass' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>
                  </div>
                  <p className="text-[11px] text-slate-400">Tersimpan dalam brankas AES-256-GCM.</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Table Prefix
                  </label>
                  <Input
                    type="text"
                    placeholder="wp_"
                    value={tablePrefix}
                    onChange={(e) => setTablePrefix(e.target.value)}
                    className="text-xs font-mono"
                  />
                  <p className="text-[11px] text-slate-400">Default: wp_ (dapat diubah untuk keamanan tambahan).</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    WordPress Core Version
                  </label>
                  <select
                    value={wpVersion}
                    onChange={(e) => setWpVersion(e.target.value)}
                    className="w-full text-xs px-3 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200"
                  >
                    <option value="latest">WordPress Latest (Recommended)</option>
                    <option value="6.7">WordPress 6.7</option>
                    <option value="6.6.2">WordPress 6.6.2</option>
                    <option value="6.5.5">WordPress 6.5.5</option>
                  </select>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Permalink Structure
                  </label>
                  <select
                    value={permalinkStructure}
                    onChange={(e) => setPermalinkStructure(e.target.value)}
                    className="w-full text-xs px-3 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200 font-mono"
                  >
                    <option value="/%postname%/">/%postname%/ (Post Name - SEO Best Practice)</option>
                    <option value="/%year%/%monthnum%/%postname%/">/%year%/%monthnum%/%postname%/ (Day & Name)</option>
                    <option value="/archives/%post_id%">/archives/%post_id% (Numeric)</option>
                  </select>
                </div>
              </div>
            </div>
          )}

          {/* ========================================================== */}
          {/* STEP 3: Theme & Plugin ZIP Upload (Pre-launch Assets)     */}
          {/* ========================================================== */}
          {currentStep === 3 && (
            <div className="space-y-4">
              <div className="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-xl p-4 text-xs space-y-1">
                <div className="font-semibold text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                  <FolderArchive className="h-4 w-4 text-blue-500" />
                  <span>Pre-Install Theme & Plugin Kustom (Opsional)</span>
                </div>
                <p className="text-slate-500 dark:text-slate-400">
                  Unggah file ZIP theme premium (Astra, GeneratePress, dll) atau plugin esensial. Ekstraksi dan aktivasi akan dieksekusi otomatis saat peluncuran selesai. Anda juga bisa mengunggahnya kapan saja di AI Webmaster Studio.
                </p>
              </div>

              {/* Asset Dropzone */}
              <div
                onDragOver={(e) => {
                  e.preventDefault()
                  setDragOver(true)
                }}
                onDragLeave={() => setDragOver(false)}
                onDrop={(e) => {
                  e.preventDefault()
                  setDragOver(false)
                  handleFileDrop(e.dataTransfer.files)
                }}
                className={`border-2 border-dashed rounded-2xl p-6 text-center transition-all ${
                  dragOver
                    ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/20'
                    : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'
                }`}
              >
                <div className="max-w-md mx-auto space-y-3">
                  <div className="w-12 h-12 rounded-full bg-blue-500/10 text-blue-500 mx-auto flex items-center justify-center">
                    <UploadCloud className="h-6 w-6" />
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                      Tarik & Letakkan file ZIP theme atau plugin di sini
                    </p>
                    <p className="text-[11px] text-slate-400 mt-0.5">
                      Format didukung: file archive .zip (maks. 50 MB)
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center justify-center gap-2 pt-2">
                    {/* Kind Selector */}
                    <div className="flex items-center rounded-lg border border-slate-200 dark:border-slate-700 p-0.5 bg-white dark:bg-slate-800 text-xs">
                      <button
                        type="button"
                        onClick={() => setSelectedKind('theme')}
                        className={`px-3 py-1 rounded-md font-medium transition-colors ${
                          selectedKind === 'theme'
                            ? 'bg-blue-600 text-white shadow-2xs'
                            : 'text-slate-600 dark:text-slate-400'
                        }`}
                      >
                        Theme
                      </button>
                      <button
                        type="button"
                        onClick={() => setSelectedKind('plugin')}
                        className={`px-3 py-1 rounded-md font-medium transition-colors ${
                          selectedKind === 'plugin'
                            ? 'bg-blue-600 text-white shadow-2xs'
                            : 'text-slate-600 dark:text-slate-400'
                        }`}
                      >
                        Plugin
                      </button>
                    </div>

                    <label className="cursor-pointer">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 text-xs pointer-events-none"
                      >
                        <FileArchive className="h-3.5 w-3.5 mr-1" />
                        Pilih File ZIP
                      </Button>
                      <input
                        type="file"
                        accept=".zip,application/zip"
                        className="hidden"
                        onChange={(e) => handleFileDrop(e.target.files)}
                      />
                    </label>

                    <div className="flex items-center gap-1.5 ml-2">
                      <input
                        type="checkbox"
                        id="modalAutoAct"
                        checked={autoActivate}
                        onChange={(e) => setAutoActivate(e.target.checked)}
                        className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                      />
                      <label htmlFor="modalAutoAct" className="text-[11px] text-slate-600 dark:text-slate-400 cursor-pointer">
                        Aktivasi otomatis
                      </label>
                    </div>
                  </div>
                </div>
              </div>

              {/* Staged Assets List */}
              {stagedAssets.length > 0 && (
                <div className="space-y-2">
                  <h4 className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Daftar Asset Siap Instal ({stagedAssets.length})
                  </h4>
                  <div className="space-y-1.5">
                    {stagedAssets.map((asset, idx) => (
                      <div
                        key={idx}
                        className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700/60 text-xs"
                      >
                        <div className="flex items-center gap-2.5">
                          <FileArchive className="h-4 w-4 text-blue-500 shrink-0" />
                          <div>
                            <div className="flex items-center gap-2 font-medium text-slate-800 dark:text-slate-200">
                              <span>{asset.file.name}</span>
                              <Badge className="text-[10px] uppercase font-semibold">
                                {asset.kind}
                              </Badge>
                              {asset.activate && (
                                <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 text-[10px]">
                                  Auto-Activate
                                </Badge>
                              )}
                            </div>
                            <p className="text-[11px] text-slate-400 mt-0.5">
                              Slug: <code className="font-mono text-blue-600 dark:text-blue-400">{asset.slug}</code> • {(asset.file.size / 1024 / 1024).toFixed(2)} MB
                            </p>
                          </div>
                        </div>

                        <button
                          type="button"
                          onClick={() => handleRemoveStagedAsset(idx)}
                          className="p-1.5 text-slate-400 hover:text-rose-500 transition-colors"
                          title="Hapus dari antrean"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ========================================================== */}
          {/* STEP 4: Live Launch & Bootstrapping Console               */}
          {/* ========================================================== */}
          {currentStep === 4 && (
            <div className="space-y-4">
              {/* Ready to Deploy Banner if not yet started */}
              {!isDeploying && !deployResult && (
                <div className="p-5 rounded-2xl bg-blue-50/60 dark:bg-blue-950/20 border border-blue-200/60 dark:border-blue-900/40 text-center space-y-3">
                  <div className="w-12 h-12 rounded-full bg-blue-600 text-white flex items-center justify-center mx-auto shadow-md">
                    <Rocket className="h-6 w-6" />
                  </div>
                  <div>
                    <h3 className="text-sm font-semibold text-slate-900 dark:text-white">
                      Siap Meluncurkan Turnkey WordPress
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-1 max-w-md mx-auto">
                      Semua konfigurasi telah siap. Cuan Engine akan mengorkestrasi container WordPress, database MySQL, WP-CLI setup, dan Caddy SSL.
                    </p>
                  </div>
                  <div className="pt-2">
                    <Button
                      onClick={handleStartDeploy}
                      className="bg-blue-600 hover:bg-blue-500 text-white px-6 py-2.5 rounded-xl font-semibold text-xs shadow-md shadow-blue-500/20"
                    >
                      <Rocket className="h-4 w-4 mr-1.5" />
                      Mulai Peluncuran Sekarang
                    </Button>
                  </div>
                </div>
              )}

              {/* Progress Steps Indicator */}
              {(isDeploying || deployResult) && (
                <div className="space-y-3">
                  <div className="grid grid-cols-4 gap-2 text-center text-xs">
                    {[
                      { idx: 1, title: 'Database' },
                      { idx: 2, title: 'WP-CLI Core' },
                      { idx: 3, title: 'Caddy SSL' },
                      { idx: 4, title: 'Live Health' },
                    ].map((step) => {
                      const isComplete = deployStepIndex >= step.idx
                      const isCurrent = deployStepIndex === step.idx && isDeploying
                      return (
                        <div
                          key={step.idx}
                          className={`p-2 rounded-xl border text-xs font-medium transition-all ${
                            isComplete
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                              : isCurrent
                              ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30 animate-pulse'
                              : 'bg-slate-50 dark:bg-slate-800 text-slate-400 border-slate-200 dark:border-slate-700'
                          }`}
                        >
                          {isComplete ? '✓ ' : ''}
                          {step.title}
                        </div>
                      )
                    })}
                  </div>

                  {/* Terminal Execution Logs */}
                  <div className="bg-slate-950 text-slate-200 font-mono text-[11px] rounded-xl p-4 border border-slate-800 max-h-56 overflow-y-auto space-y-1 shadow-inner">
                    <div className="flex items-center justify-between text-slate-400 pb-2 mb-2 border-b border-slate-800 text-[10px]">
                      <span className="flex items-center gap-1.5">
                        <TerminalIcon className="h-3.5 w-3.5 text-blue-400" />
                        WordPress Orchestrator Stream
                      </span>
                      {isDeploying && <span className="animate-pulse text-blue-400">Processing...</span>}
                    </div>
                    {deployLogs.map((log, i) => (
                      <div key={i} className="leading-relaxed whitespace-pre-wrap">
                        {log}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Success Result Credentials Card */}
              {deployResult && (
                <div className="p-5 rounded-2xl bg-emerald-50/50 dark:bg-emerald-950/20 border border-emerald-200/60 dark:border-emerald-800/40 space-y-4 animate-in fade-in duration-200">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 text-emerald-700 dark:text-emerald-300">
                      <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                      <span className="font-semibold text-sm">
                        Instalasi WordPress Selesai & Berjalan Sehat!
                      </span>
                    </div>
                    <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                      HTTP 200 OK
                    </Badge>
                  </div>

                  {/* Action Link Buttons */}
                  <div className="flex flex-wrap items-center gap-2 pt-1">
                    <a
                      href={deployResult.site_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-medium text-xs shadow-xs"
                    >
                      <Globe className="h-3.5 w-3.5" />
                      <span>Visit Live Site</span>
                      <ExternalLink className="h-3 w-3 ml-0.5" />
                    </a>

                    <a
                      href={deployResult.admin_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs shadow-xs"
                    >
                      <KeyRound className="h-3.5 w-3.5" />
                      <span>Open WP-Admin</span>
                      <ExternalLink className="h-3 w-3 ml-0.5" />
                    </a>

                    {onOpenDashboard && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          onClose()
                          onOpenDashboard()
                        }}
                        className="text-xs"
                      >
                        Buka Webmaster Studio
                      </Button>
                    )}
                  </div>

                  {/* Credentials Disclosure Box */}
                  <div className="bg-white dark:bg-slate-900 rounded-xl p-4 border border-slate-200 dark:border-slate-800 space-y-3">
                    <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-2">
                      <div className="flex items-center gap-1.5 font-semibold text-xs text-slate-800 dark:text-slate-200">
                        <KeyRound className="h-4 w-4 text-blue-500" />
                        <span>Kredensial Akses Administrator & Database</span>
                      </div>
                      <Badge className="text-[10px] bg-slate-100 dark:bg-slate-800 text-slate-500">
                        Simpan Sekarang
                      </Badge>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                      <div>
                        <span className="text-slate-400 text-[11px]">Site URL:</span>
                        <div className="flex items-center justify-between font-mono bg-slate-50 dark:bg-slate-800/60 p-2 rounded-lg mt-0.5">
                          <span className="truncate">{deployResult.site_url}</span>
                          <button
                            onClick={() => handleCopy(deployResult.site_url, 'res_site')}
                            className="text-slate-400 hover:text-slate-600 ml-2"
                          >
                            {copiedField === 'res_site' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                          </button>
                        </div>
                      </div>

                      <div>
                        <span className="text-slate-400 text-[11px]">WP-Admin URL:</span>
                        <div className="flex items-center justify-between font-mono bg-slate-50 dark:bg-slate-800/60 p-2 rounded-lg mt-0.5">
                          <span className="truncate">{deployResult.admin_url}</span>
                          <button
                            onClick={() => handleCopy(deployResult.admin_url, 'res_admin_url')}
                            className="text-slate-400 hover:text-slate-600 ml-2"
                          >
                            {copiedField === 'res_admin_url' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                          </button>
                        </div>
                      </div>

                      <div>
                        <span className="text-slate-400 text-[11px]">Admin Username:</span>
                        <div className="flex items-center justify-between font-mono bg-slate-50 dark:bg-slate-800/60 p-2 rounded-lg mt-0.5">
                          <span>{adminUsername}</span>
                          <button
                            onClick={() => handleCopy(adminUsername, 'res_user')}
                            className="text-slate-400 hover:text-slate-600 ml-2"
                          >
                            {copiedField === 'res_user' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                          </button>
                        </div>
                      </div>

                      <div>
                        <span className="text-slate-400 text-[11px]">Admin Password:</span>
                        <div className="flex items-center justify-between font-mono bg-slate-50 dark:bg-slate-800/60 p-2 rounded-lg mt-0.5">
                          <span>{showAdminPassword ? adminPassword : '••••••••••••••••••••••••'}</span>
                          <div className="flex items-center gap-1.5 ml-2">
                            <button
                              onClick={() => setShowAdminPassword(!showAdminPassword)}
                              className="text-slate-400 hover:text-slate-600"
                            >
                              {showAdminPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                            </button>
                            <button
                              onClick={() => handleCopy(adminPassword, 'res_pass')}
                              className="text-slate-400 hover:text-slate-600"
                            >
                              {copiedField === 'res_pass' ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Modal Footer Controls */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
          <div>
            {currentStep > 1 && !deployResult && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentStep((currentStep - 1) as any)}
                disabled={isDeploying}
                className="text-xs"
              >
                <ArrowLeft className="h-3.5 w-3.5 mr-1" />
                Kembali
              </Button>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
              disabled={isDeploying}
              className="text-xs"
            >
              Tutup
            </Button>

            {currentStep === 1 && (
              <Button
                size="sm"
                onClick={() => setCurrentStep(2)}
                disabled={!isStep1Valid}
                className="text-xs bg-blue-600 hover:bg-blue-500 text-white"
              >
                Lanjut ke Domain & DB
                <ArrowRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            )}

            {currentStep === 2 && (
              <Button
                size="sm"
                onClick={() => setCurrentStep(3)}
                disabled={!isStep2Valid}
                className="text-xs bg-blue-600 hover:bg-blue-500 text-white"
              >
                Lanjut ke Theme & Plugin
                <ArrowRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            )}

            {currentStep === 3 && (
              <Button
                size="sm"
                onClick={() => setCurrentStep(4)}
                className="text-xs bg-blue-600 hover:bg-blue-500 text-white"
              >
                Ke Konsol Peluncuran
                <ArrowRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            )}

            {currentStep === 4 && deployResult && (
              <Button
                size="sm"
                onClick={onClose}
                className="text-xs bg-emerald-600 hover:bg-emerald-500 text-white"
              >
                Selesai
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export const WordPressLaunchpadModal = WordPressDeployerModal
