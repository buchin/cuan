import { useState, useEffect, useMemo } from 'react'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Badge } from '../../ui/badge'
import {
  X,
  KeyRound,
  Database,
  Server,
  Terminal as TerminalIcon,
  Sparkles,
  CheckCircle2,
  AlertCircle,
  Eye,
  EyeOff,
  Plus,
  Trash2,
  ExternalLink,
  RefreshCw,
  Rocket,
  ArrowRight,
  ArrowLeft,
  Globe,
  FileCode,
  ShieldCheck
} from 'lucide-react'
import {
  ScriptDeploymentDTO,
  ProvisionDBResponse,
  GenerateEnvResponse,
  GetProjectEnvResponse,
  ScriptDeployResponse,
  ScriptDeployStatusResponse,
  SupervisedProcessStatus,
  ServerItem
} from '../types'
import { ScriptDeployerConsole } from './ScriptDeployerConsole'

export interface ScriptDeployerModalProps {
  isOpen: boolean
  onClose: () => void
  projectId: string
  projectName?: string
  projectSlug?: string
  baseDomain?: string
  servers?: ServerItem[]
  initialStep?: 1 | 2 | 3 | 4
  onDeploymentSuccess?: (deployment: ScriptDeploymentDTO) => void
}

interface EnvVarRow {
  key: string
  value: string
  isSecret?: boolean
  showPlain?: boolean
}

export function ScriptDeployerModal({
  isOpen,
  onClose,
  projectId,
  projectName = 'Script Project',
  projectSlug = '',
  baseDomain = 'cuan.dojo.cc',
  servers = [],
  initialStep = 1,
  onDeploymentSuccess,
}: ScriptDeployerModalProps) {
  // Wizard current step: 1 | 2 | 3 | 4
  const [currentStep, setCurrentStep] = useState<1 | 2 | 3 | 4>(initialStep)

  // Feedback notifications
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // -------------------------------------------------------------
  // Step 1: Environment & Secrets State
  // -------------------------------------------------------------
  const [envVars, setEnvVars] = useState<EnvVarRow[]>([])
  const [loadingEnv, setLoadingEnv] = useState(false)
  const [generatingSecrets, setGeneratingSecrets] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [showAllSecrets, setShowAllSecrets] = useState(false)

  // Check if a key looks like a secret
  const isKeySecret = (key: string): boolean => {
    const lower = key.toLowerCase()
    return (
      lower.includes('secret') ||
      lower.includes('password') ||
      lower.includes('key') ||
      lower.includes('token') ||
      lower.includes('salt') ||
      lower.includes('auth')
    )
  }

  // Load existing env on modal open
  const fetchProjectEnv = async () => {
    setLoadingEnv(true)
    setErrorMsg(null)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/env`)
      if (res.ok) {
        const data: GetProjectEnvResponse = await res.json()
        const rows: EnvVarRow[] = []
        if (data.masked_env) {
          Object.entries(data.masked_env).forEach(([k, v]) => {
            rows.push({
              key: k,
              value: v,
              isSecret: isKeySecret(k),
              showPlain: false,
            })
          })
        }
        setEnvVars(rows)
      }
    } catch (err: any) {
      setErrorMsg(err.message || 'Gagal mengambil konfigurasi environment.')
    } finally {
      setLoadingEnv(false)
    }
  }

  // Auto-generate secrets via POST /api/v1/projects/{id}/generate-env
  const handleGenerateSecrets = async () => {
    setGeneratingSecrets(true)
    setErrorMsg(null)
    setSuccessMsg(null)

    try {
      // Collect custom vars from current table
      const customVars: Record<string, string> = {}
      envVars.forEach((row) => {
        if (row.key.trim() && !row.value.includes('****')) {
          customVars[row.key.trim()] = row.value
        }
      })

      const res = await fetch(`/api/v1/projects/${projectId}/generate-env`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          custom_vars: customVars,
        }),
      })

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || `HTTP ${res.status}: Gagal men-generate secrets`)
      }

      const data: GenerateEnvResponse = await res.json()
      const updatedRows: EnvVarRow[] = []
      if (data.masked_env) {
        Object.entries(data.masked_env).forEach(([k, v]) => {
          updatedRows.push({
            key: k,
            value: v,
            isSecret: isKeySecret(k),
            showPlain: false,
          })
        })
      }
      setEnvVars(updatedRows)
      setSuccessMsg('🎉 Secrets & Environment berhasil di-generate dan diamankan dengan enkripsi AES-256-GCM!')
    } catch (err: any) {
      setErrorMsg(err.message || 'Gagal men-generate secrets aplikasi.')
    } finally {
      setGeneratingSecrets(false)
    }
  }

  const handleAddEnvVar = () => {
    if (!newKey.trim()) return
    const key = newKey.trim().toUpperCase().replace(/[^A-Z0-9_]/g, '_')
    const existingIdx = envVars.findIndex((r) => r.key === key)

    if (existingIdx >= 0) {
      setEnvVars((prev) =>
        prev.map((r, i) => (i === existingIdx ? { ...r, value: newValue.trim() } : r))
      )
    } else {
      setEnvVars((prev) => [
        ...prev,
        {
          key,
          value: newValue.trim(),
          isSecret: isKeySecret(key),
          showPlain: false,
        },
      ])
    }
    setNewKey('')
    setNewValue('')
  }

  const handleDeleteEnvVar = (key: string) => {
    setEnvVars((prev) => prev.filter((r) => r.key !== key))
  }

  const handleToggleRowSecret = (idx: number) => {
    setEnvVars((prev) =>
      prev.map((r, i) => (i === idx ? { ...r, showPlain: !r.showPlain } : r))
    )
  }

  // -------------------------------------------------------------
  // Step 2: Database Provisioning State
  // -------------------------------------------------------------
  const [dbEngine, setDbEngine] = useState<'sqlite' | 'mysql' | 'postgres'>('sqlite')
  const [dbName, setDbName] = useState(() => {
    const slug = (projectSlug || projectId).replace(/[^a-zA-Z0-9_]/g, '_').toLowerCase()
    return `cuan_${slug}`
  })
  const [dbUser, setDbUser] = useState(() => {
    const slug = (projectSlug || projectId).replace(/[^a-zA-Z0-9_]/g, '_').toLowerCase()
    return `usr_${slug}`
  })
  const [dbPassword, setDbPassword] = useState('')
  const [sqlFile, setSqlFile] = useState('database.sql')
  const [importSchema, setImportSchema] = useState(true)
  const [isProvisioning, setIsProvisioning] = useState(false)
  const [provisionResult, setProvisionResult] = useState<ProvisionDBResponse | null>(null)

  const handleProvisionDB = async () => {
    setIsProvisioning(true)
    setErrorMsg(null)
    setSuccessMsg(null)

    try {
      const res = await fetch(`/api/v1/projects/${projectId}/provision-db`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          engine: dbEngine,
          db_name: dbName.trim(),
          db_user: dbUser.trim(),
          db_password: dbPassword.trim(),
          sql_file: sqlFile.trim(),
          import_schema: importSchema,
        }),
      })

      const data: ProvisionDBResponse = await res.json()
      if (!res.ok) {
        const errData = data as any
        throw new Error(errData.error || `HTTP ${res.status}: Gagal mem-provision database`)
      }

      setProvisionResult(data)
      setSuccessMsg(
        `✅ Database ${data.database.engine.toUpperCase()} "${data.database.db_name}" berhasil diprovision! ${data.tables_count} tabel terimpor (${data.total_statements} statements).`
      )

      // Auto refresh env after DB provision to incorporate DB vars
      fetchProjectEnv()
    } catch (err: any) {
      setErrorMsg(err.message || 'Terjadi kesalahan saat provisioning database.')
    } finally {
      setIsProvisioning(false)
    }
  }

  // -------------------------------------------------------------
  // Step 3: Domain & Server Target State
  // -------------------------------------------------------------
  const computedDefaultSubdomain = useMemo(() => {
    const clean = (projectSlug || projectName || projectId)
      .toLowerCase()
      .replace(/[^a-z0-9-]/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
    return `${clean}.${baseDomain}`
  }, [projectSlug, projectName, projectId, baseDomain])

  const [domainType, setDomainType] = useState<'subdomain' | 'custom'>('subdomain')
  const [domainValue, setDomainValue] = useState(computedDefaultSubdomain)
  const [selectedServerId, setSelectedServerId] = useState<string>('')
  const [runInstall, setRunInstall] = useState(true)
  const [runBuild, setRunBuild] = useState(true)
  const [runMigrate, setRunMigrate] = useState(true)
  const [healthPath, setHealthPath] = useState('/')
  const [timeoutSeconds, setTimeoutSeconds] = useState(600)

  // -------------------------------------------------------------
  // Step 4: Live Build & Deploy State
  // -------------------------------------------------------------
  const [isDeploying, setIsDeploying] = useState(false)
  const [activeDeployment, setActiveDeployment] = useState<ScriptDeploymentDTO | null>(null)
  const [activeProcess, setActiveProcess] = useState<SupervisedProcessStatus | null>(null)

  // Fetch status on mount or when in step 4
  const fetchDeployStatus = async () => {
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/script-deploy/status`)
      if (res.ok) {
        const data: ScriptDeployStatusResponse = await res.json()
        if (data.deployment) setActiveDeployment(data.deployment)
        if (data.process) setActiveProcess(data.process)
      }
    } catch {}
  }

  // Trigger Script Deploy
  const handleTriggerDeploy = async () => {
    setIsDeploying(true)
    setErrorMsg(null)
    setCurrentStep(4) // Move to Live Build Console immediately

    try {
      const payload = {
        server_id: selectedServerId ? selectedServerId : undefined,
        domain: domainValue.trim(),
        run_install: runInstall,
        run_build: runBuild,
        run_migrate: runMigrate,
        health_path: healthPath.trim() || '/',
        timeout_seconds: timeoutSeconds,
      }

      const res = await fetch(`/api/v1/projects/${projectId}/script-deploy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      const data: ScriptDeployResponse = await res.json()
      if (!res.ok) {
        const errData = data as any
        throw new Error(errData.error || `HTTP ${res.status}: Gagal mengeksekusi pipeline deploy`)
      }

      setActiveDeployment(data.deployment)
      setActiveProcess(data.process)
      onDeploymentSuccess?.(data.deployment)
      setSuccessMsg('🚀 Script berhasil di-deploy dan service supervisor berjalan!')
    } catch (err: any) {
      setErrorMsg(err.message || 'Pipeline build runner gagal.')
    } finally {
      setIsDeploying(false)
    }
  }

  // Restart Service
  const handleRestartService = async () => {
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/script-deploy/restart`, {
        method: 'POST',
      })
      if (res.ok) {
        const data = await res.json()
        if (data.deployment) setActiveDeployment(data.deployment)
        if (data.process) setActiveProcess(data.process)
        setSuccessMsg('Service berhasil di-restart.')
      } else {
        const err = await res.json().catch(() => ({}))
        setErrorMsg(err.error || 'Gagal me-restart service.')
      }
    } catch (err: any) {
      setErrorMsg(err.message || 'Gagal me-restart service.')
    }
  }

  // Init when modal opens
  useEffect(() => {
    if (isOpen) {
      setCurrentStep(initialStep)
      fetchProjectEnv()
      fetchDeployStatus()
    }
  }, [isOpen, projectId, initialStep])

  if (!isOpen) return null

  // Live app URL calculation
  const liveAppUrl =
    (activeDeployment?.domain ? `https://${activeDeployment.domain}` : '') ||
    (activeProcess?.domain ? `https://${activeProcess.domain}` : '') ||
    (activeDeployment?.port ? `http://127.0.0.1:${activeDeployment.port}` : '')

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl w-full max-w-4xl shadow-2xl flex flex-col max-h-[92vh] overflow-hidden">
        {/* Modal Top Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-2xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-600 dark:text-cyan-400 flex items-center justify-center shrink-0">
              <Rocket className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base sm:text-lg font-bold text-slate-900 dark:text-white tracking-tight">
                  1-Click Script Deployer Wizard
                </h2>
                <Badge variant="outline" className="text-[10px] text-cyan-600 border-cyan-500/30">
                  Sprint 12
                </Badge>
              </div>
              <p className="text-xs text-slate-400 truncate max-w-md">
                Project: <span className="font-semibold text-slate-600 dark:text-slate-300">{projectName}</span> ({projectId})
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-2 rounded-xl text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Wizard Multi-Step Progress Stepper */}
        <div className="grid grid-cols-4 border-b border-slate-200 dark:border-slate-800 bg-slate-100/60 dark:bg-slate-950/60 text-xs">
          {/* Step 1 Tab */}
          <button
            onClick={() => setCurrentStep(1)}
            className={`py-3 px-2 flex items-center justify-center gap-2 font-semibold transition-all border-b-2 ${
              currentStep === 1
                ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400 bg-white dark:bg-slate-900 shadow-xs'
                : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'
            }`}
          >
            <KeyRound className="h-4 w-4" />
            <span className="hidden sm:inline">1. Environment</span>
          </button>

          {/* Step 2 Tab */}
          <button
            onClick={() => setCurrentStep(2)}
            className={`py-3 px-2 flex items-center justify-center gap-2 font-semibold transition-all border-b-2 ${
              currentStep === 2
                ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400 bg-white dark:bg-slate-900 shadow-xs'
                : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'
            }`}
          >
            <Database className="h-4 w-4" />
            <span className="hidden sm:inline">2. Provision DB</span>
          </button>

          {/* Step 3 Tab */}
          <button
            onClick={() => setCurrentStep(3)}
            className={`py-3 px-2 flex items-center justify-center gap-2 font-semibold transition-all border-b-2 ${
              currentStep === 3
                ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400 bg-white dark:bg-slate-900 shadow-xs'
                : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'
            }`}
          >
            <Server className="h-4 w-4" />
            <span className="hidden sm:inline">3. Server & Domain</span>
          </button>

          {/* Step 4 Tab */}
          <button
            onClick={() => setCurrentStep(4)}
            className={`py-3 px-2 flex items-center justify-center gap-2 font-semibold transition-all border-b-2 ${
              currentStep === 4
                ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400 bg-white dark:bg-slate-900 shadow-xs'
                : 'border-transparent text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'
            }`}
          >
            <TerminalIcon className="h-4 w-4" />
            <span className="hidden sm:inline">4. Live Console</span>
          </button>
        </div>

        {/* Global Alert Notification */}
        {errorMsg && (
          <div className="mx-6 mt-4 p-3 rounded-xl bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900 text-rose-700 dark:text-rose-300 text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>{errorMsg}</span>
            </div>
            <button onClick={() => setErrorMsg(null)} className="opacity-80 hover:opacity-100">
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        {successMsg && (
          <div className="mx-6 mt-4 p-3 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 text-emerald-700 dark:text-emerald-300 text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 shrink-0" />
              <span>{successMsg}</span>
            </div>
            <button onClick={() => setSuccessMsg(null)} className="opacity-80 hover:opacity-100">
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        {/* Modal Scrollable Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* ============================================================== */}
          {/* STEP 1: Environment & Secrets Vault */}
          {/* ============================================================== */}
          {currentStep === 1 && (
            <div className="space-y-5 animate-in fade-in duration-200">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <KeyRound className="h-4 w-4 text-cyan-500" />
                    <span>Konfigurasi Environment & Secrets (.env)</span>
                  </h3>
                  <p className="text-xs text-slate-400">
                    Nilai sensitif diamankan dengan enkripsi AES-256-GCM. Gunakan fitur auto-generate untuk membuat secret keys acak berkeamanan tinggi.
                  </p>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    onClick={handleGenerateSecrets}
                    disabled={generatingSecrets}
                    className="h-8 text-xs font-semibold bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white gap-1.5 shadow-md shadow-cyan-500/20"
                  >
                    <Sparkles className={`h-3.5 w-3.5 ${generatingSecrets ? 'animate-spin' : ''}`} />
                    <span>{generatingSecrets ? 'Generating...' : 'Auto-Generate Secrets'}</span>
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowAllSecrets(!showAllSecrets)}
                    className="h-8 text-xs gap-1 border-slate-300 dark:border-slate-700"
                    title={showAllSecrets ? 'Sembunyikan Nilai Sensitif' : 'Tampilkan Nilai Sensitif'}
                  >
                    {showAllSecrets ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                  </Button>
                </div>
              </div>

              {/* Add New Variable Input Bar */}
              <div className="p-3 bg-slate-50 dark:bg-slate-950/60 rounded-xl border border-slate-200 dark:border-slate-800 flex flex-wrap items-center gap-2 text-xs">
                <Input
                  type="text"
                  placeholder="KEY (e.g. STRIPE_KEY, REDIS_HOST)"
                  value={newKey}
                  onChange={(e) => setNewKey(e.target.value)}
                  className="h-8 text-xs font-mono w-48 sm:w-64"
                />
                <Input
                  type="text"
                  placeholder="Value..."
                  value={newValue}
                  onChange={(e) => setNewValue(e.target.value)}
                  className="h-8 text-xs font-mono flex-1 min-w-[160px]"
                />
                <Button
                  size="sm"
                  onClick={handleAddEnvVar}
                  className="h-8 text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-white dark:bg-slate-100 dark:hover:bg-slate-200 dark:text-slate-900 gap-1"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Tambah Variable</span>
                </Button>
              </div>

              {/* Environment Variables Table */}
              <div className="border border-slate-200 dark:border-slate-800 rounded-2xl overflow-hidden bg-white dark:bg-slate-950">
                <div className="max-h-72 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800 text-xs">
                  {loadingEnv ? (
                    <div className="p-8 text-center text-slate-400 flex flex-col items-center gap-2">
                      <RefreshCw className="h-5 w-5 animate-spin text-cyan-500" />
                      <span>Memuat konfigurasi environment...</span>
                    </div>
                  ) : envVars.length === 0 ? (
                    <div className="p-8 text-center text-slate-400 space-y-2">
                      <p>Belum ada variabel environment tersimpan.</p>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={handleGenerateSecrets}
                        className="text-xs gap-1 border-dashed"
                      >
                        <Sparkles className="h-3.5 w-3.5 text-cyan-500" />
                        <span>Generate Nilai Default & Secrets</span>
                      </Button>
                    </div>
                  ) : (
                    envVars.map((row, idx) => (
                      <div
                        key={row.key}
                        className="px-4 py-2.5 flex items-center justify-between gap-3 hover:bg-slate-50/60 dark:hover:bg-slate-900/60 transition-colors"
                      >
                        <div className="flex items-center gap-2 font-mono font-bold text-slate-700 dark:text-slate-200 min-w-[160px] truncate">
                          {row.isSecret ? (
                            <ShieldCheck className="h-3.5 w-3.5 text-cyan-500 shrink-0" />
                          ) : (
                            <FileCode className="h-3.5 w-3.5 text-slate-400 shrink-0" />
                          )}
                          <span title={row.key}>{row.key}</span>
                        </div>

                        <div className="flex-1 font-mono text-slate-600 dark:text-slate-400 truncate">
                          {row.isSecret && !row.showPlain && !showAllSecrets ? (
                            <span className="tracking-widest text-slate-400">••••••••••••••••</span>
                          ) : (
                            <span>{row.value}</span>
                          )}
                        </div>

                        <div className="flex items-center gap-1">
                          {row.isSecret && (
                            <button
                              onClick={() => handleToggleRowSecret(idx)}
                              className="p-1 rounded text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                              title={row.showPlain ? 'Mask secret' : 'Reveal secret'}
                            >
                              {row.showPlain || showAllSecrets ? (
                                <EyeOff className="h-3.5 w-3.5" />
                              ) : (
                                <Eye className="h-3.5 w-3.5" />
                              )}
                            </button>
                          )}
                          <button
                            onClick={() => handleDeleteEnvVar(row.key)}
                            className="p-1 rounded text-slate-400 hover:text-rose-500"
                            title="Hapus variabel"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>

              {/* Step 1 Footer Action */}
              <div className="flex justify-end pt-2">
                <Button
                  onClick={() => setCurrentStep(2)}
                  className="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold gap-1.5 shadow-sm"
                >
                  <span>Lanjut: Database Provisioning</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )}

          {/* ============================================================== */}
          {/* STEP 2: Database Provisioning & Schema Import */}
          {/* ============================================================== */}
          {currentStep === 2 && (
            <div className="space-y-5 animate-in fade-in duration-200">
              <div>
                <h3 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                  <Database className="h-4 w-4 text-cyan-500" />
                  <span>Automated Database Provisioning & SQL Importer</span>
                </h3>
                <p className="text-xs text-slate-400">
                  Cuan membuat database terisolasi dan mengeksekusi skema `.sql` secara otomatis tanpa perlu membuka phpMyAdmin atau terminal CLI.
                </p>
              </div>

              {/* Database Engine Picker */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {/* SQLite Option */}
                <div
                  onClick={() => setDbEngine('sqlite')}
                  className={`p-4 rounded-2xl border cursor-pointer transition-all ${
                    dbEngine === 'sqlite'
                      ? 'border-cyan-500 bg-cyan-500/5 dark:bg-cyan-950/20 ring-2 ring-cyan-500/20'
                      : 'border-slate-200 dark:border-slate-800 hover:border-slate-300'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-bold text-xs text-slate-900 dark:text-white">SQLite</span>
                    <Badge variant="outline" className="text-[10px]">Zero-Config</Badge>
                  </div>
                  <p className="text-[11px] text-slate-400">
                    File-based database terisolasi. Cocok untuk prototype, blog statis, dan script ringan.
                  </p>
                </div>

                {/* MySQL Option */}
                <div
                  onClick={() => setDbEngine('mysql')}
                  className={`p-4 rounded-2xl border cursor-pointer transition-all ${
                    dbEngine === 'mysql'
                      ? 'border-cyan-500 bg-cyan-500/5 dark:bg-cyan-950/20 ring-2 ring-cyan-500/20'
                      : 'border-slate-200 dark:border-slate-800 hover:border-slate-300'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-bold text-xs text-slate-900 dark:text-white">MySQL / MariaDB</span>
                    <Badge variant="outline" className="text-[10px]">PHP / Laravel</Badge>
                  </div>
                  <p className="text-[11px] text-slate-400">
                    Database relasional standar untuk kebanyakan script CodeCanyon & PHP/WordPress.
                  </p>
                </div>

                {/* PostgreSQL Option */}
                <div
                  onClick={() => setDbEngine('postgres')}
                  className={`p-4 rounded-2xl border cursor-pointer transition-all ${
                    dbEngine === 'postgres'
                      ? 'border-cyan-500 bg-cyan-500/5 dark:bg-cyan-950/20 ring-2 ring-cyan-500/20'
                      : 'border-slate-200 dark:border-slate-800 hover:border-slate-300'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-bold text-xs text-slate-900 dark:text-white">PostgreSQL</span>
                    <Badge variant="outline" className="text-[10px]">Node / Python</Badge>
                  </div>
                  <p className="text-[11px] text-slate-400">
                    Database relasional enterprise dengan performa tinggi & isolasi schema fleksibel.
                  </p>
                </div>
              </div>

              {/* Database Credentials Form */}
              <div className="p-4 bg-slate-50 dark:bg-slate-950/60 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3 text-xs">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                      Nama Database
                    </label>
                    <Input
                      type="text"
                      value={dbName}
                      onChange={(e) => setDbName(e.target.value)}
                      className="h-8 text-xs font-mono"
                      placeholder="cuan_db"
                    />
                  </div>

                  <div>
                    <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                      File Skema SQL (.sql)
                    </label>
                    <Input
                      type="text"
                      value={sqlFile}
                      onChange={(e) => setSqlFile(e.target.value)}
                      className="h-8 text-xs font-mono"
                      placeholder="database.sql atau install.sql"
                    />
                  </div>
                </div>

                {dbEngine !== 'sqlite' && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                    <div>
                      <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                        DB Username
                      </label>
                      <Input
                        type="text"
                        value={dbUser}
                        onChange={(e) => setDbUser(e.target.value)}
                        className="h-8 text-xs font-mono"
                      />
                    </div>

                    <div>
                      <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                        DB Password (opsional, auto-generated jika kosong)
                      </label>
                      <Input
                        type="password"
                        value={dbPassword}
                        onChange={(e) => setDbPassword(e.target.value)}
                        className="h-8 text-xs font-mono"
                        placeholder="••••••••••••"
                      />
                    </div>
                  </div>
                )}

                <div className="flex items-center gap-2 pt-1">
                  <input
                    type="checkbox"
                    id="importSchemaCheck"
                    checked={importSchema}
                    onChange={(e) => setImportSchema(e.target.checked)}
                    className="rounded border-slate-300 text-cyan-600 focus:ring-cyan-500"
                  />
                  <label htmlFor="importSchemaCheck" className="text-xs text-slate-600 dark:text-slate-300 cursor-pointer">
                    Eksekusi impor skema `.sql` otomatis setelah database terbuat
                  </label>
                </div>
              </div>

              {/* Action Button */}
              <div className="flex items-center justify-between">
                <Button
                  variant="outline"
                  onClick={() => setCurrentStep(1)}
                  className="text-xs gap-1 border-slate-300 dark:border-slate-700"
                >
                  <ArrowLeft className="h-3.5 w-3.5" />
                  <span>Kembali</span>
                </Button>

                <div className="flex items-center gap-2">
                  <Button
                    onClick={handleProvisionDB}
                    disabled={isProvisioning}
                    className="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold gap-1.5 shadow-sm"
                  >
                    <Database className={`h-3.5 w-3.5 ${isProvisioning ? 'animate-spin' : ''}`} />
                    <span>{isProvisioning ? 'Provisioning...' : 'Provision Database Sekarang'}</span>
                  </Button>

                  <Button
                    variant="outline"
                    onClick={() => setCurrentStep(3)}
                    className="text-xs gap-1.5 border-slate-300 dark:border-slate-700"
                  >
                    <span>Lewati / Lanjut</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>

              {/* Provision Result Card */}
              {provisionResult && (
                <div className="p-4 rounded-2xl bg-emerald-500/10 border border-emerald-500/30 text-xs space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5">
                      <CheckCircle2 className="h-4 w-4" />
                      <span>Database Provisioned &amp; Ready</span>
                    </span>
                    <Badge variant="outline" className="border-emerald-500/40 text-emerald-500">
                      {provisionResult.database.engine.toUpperCase()}
                    </Badge>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                    <div>DB: {provisionResult.database.db_name}</div>
                    <div>User: {provisionResult.database.db_user || 'sqlite'}</div>
                    <div>Tables: {provisionResult.tables_count}</div>
                    <div>Statements: {provisionResult.total_statements}</div>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ============================================================== */}
          {/* STEP 3: Domain & Server Target Setup */}
          {/* ============================================================== */}
          {currentStep === 3 && (
            <div className="space-y-5 animate-in fade-in duration-200">
              <div>
                <h3 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                  <Server className="h-4 w-4 text-cyan-500" />
                  <span>Target Server &amp; Domain Setup</span>
                </h3>
                <p className="text-xs text-slate-400">
                  Konfigurasikan domain rilis dan alokasikan port otomatis bebas konflik via Caddy Reverse Proxy.
                </p>
              </div>

              {/* Domain Config */}
              <div className="p-4 bg-slate-50 dark:bg-slate-950/60 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3 text-xs">
                <div className="flex items-center gap-4">
                  <label className="flex items-center gap-2 cursor-pointer font-semibold text-slate-700 dark:text-slate-300">
                    <input
                      type="radio"
                      name="domainType"
                      checked={domainType === 'subdomain'}
                      onChange={() => {
                        setDomainType('subdomain')
                        setDomainValue(computedDefaultSubdomain)
                      }}
                      className="text-cyan-600"
                    />
                    <span>Subdomain Otomatis (*.{baseDomain})</span>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer font-semibold text-slate-700 dark:text-slate-300">
                    <input
                      type="radio"
                      name="domainType"
                      checked={domainType === 'custom'}
                      onChange={() => setDomainType('custom')}
                      className="text-cyan-600"
                    />
                    <span>Custom Domain Sendiri</span>
                  </label>
                </div>

                <div>
                  <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                    Public Release Domain
                  </label>
                  <div className="relative">
                    <Globe className="h-3.5 w-3.5 absolute left-3 top-2.5 text-slate-400" />
                    <Input
                      type="text"
                      value={domainValue}
                      onChange={(e) => setDomainValue(e.target.value)}
                      className="h-8 pl-8 text-xs font-mono"
                      placeholder="app.cuan.dojo.cc"
                    />
                  </div>
                </div>

                {/* Target Server Selection */}
                <div className="pt-2">
                  <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                    Target Server Runtime
                  </label>
                  <select
                    value={selectedServerId}
                    onChange={(e) => setSelectedServerId(e.target.value)}
                    className="w-full h-8 text-xs px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 font-semibold"
                  >
                    <option value="">Local Sandbox &amp; Internal Docker (Default Cuan Worker)</option>
                    {servers.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.host}) - {s.status}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Build Pipeline & Supervisor Options */}
              <div className="p-4 bg-slate-50 dark:bg-slate-950/60 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3 text-xs">
                <span className="font-bold text-slate-900 dark:text-white">Opsi Pipeline Runner</span>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={runInstall}
                      onChange={(e) => setRunInstall(e.target.checked)}
                      className="rounded border-slate-300 text-cyan-600"
                    />
                    <span>Install Dependencies</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={runBuild}
                      onChange={(e) => setRunBuild(e.target.checked)}
                      className="rounded border-slate-300 text-cyan-600"
                    />
                    <span>Run Build Assets</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={runMigrate}
                      onChange={(e) => setRunMigrate(e.target.checked)}
                      className="rounded border-slate-300 text-cyan-600"
                    />
                    <span>Run DB Migrations</span>
                  </label>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                  <div>
                    <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                      Health Check Path
                    </label>
                    <Input
                      type="text"
                      value={healthPath}
                      onChange={(e) => setHealthPath(e.target.value)}
                      className="h-8 text-xs font-mono"
                      placeholder="/"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-slate-600 dark:text-slate-300 mb-1">
                      Timeout (detik)
                    </label>
                    <Input
                      type="number"
                      value={timeoutSeconds}
                      onChange={(e) => setTimeoutSeconds(parseInt(e.target.value) || 600)}
                      className="h-8 text-xs font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Navigation Actions */}
              <div className="flex items-center justify-between">
                <Button
                  variant="outline"
                  onClick={() => setCurrentStep(2)}
                  className="text-xs gap-1 border-slate-300 dark:border-slate-700"
                >
                  <ArrowLeft className="h-3.5 w-3.5" />
                  <span>Kembali</span>
                </Button>

                <Button
                  onClick={handleTriggerDeploy}
                  disabled={isDeploying || !domainValue.trim()}
                  className="bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white text-xs font-bold gap-2 shadow-lg shadow-cyan-500/20 px-5"
                >
                  <Rocket className={`h-4 w-4 ${isDeploying ? 'animate-bounce' : ''}`} />
                  <span>{isDeploying ? 'Deploying...' : 'Luncurkan 1-Click Deploy!'}</span>
                </Button>
              </div>
            </div>
          )}

          {/* ============================================================== */}
          {/* STEP 4: Live Build Terminal Console */}
          {/* ============================================================== */}
          {currentStep === 4 && (
            <div className="space-y-4 animate-in fade-in duration-200">
              <ScriptDeployerConsole
                projectId={projectId}
                deployment={activeDeployment}
                process={activeProcess}
                isBuilding={isDeploying}
                onRestart={handleRestartService}
                liveAppUrl={liveAppUrl}
              />

              {/* Step 4 Footer Controls */}
              <div className="flex items-center justify-between pt-2">
                <Button
                  variant="outline"
                  onClick={() => setCurrentStep(3)}
                  className="text-xs gap-1 border-slate-300 dark:border-slate-700"
                >
                  <ArrowLeft className="h-3.5 w-3.5" />
                  <span>Konfigurasi Ulang</span>
                </Button>

                <div className="flex items-center gap-2">
                  {liveAppUrl && (
                    <a
                      href={liveAppUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-bold bg-emerald-600 hover:bg-emerald-500 text-white shadow-md shadow-emerald-600/20 transition-all"
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      <span>Open Live App</span>
                    </a>
                  )}

                  <Button
                    onClick={onClose}
                    variant="outline"
                    className="text-xs font-semibold border-slate-300 dark:border-slate-700"
                  >
                    Tutup Wizard
                  </Button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
