import { useState, useRef, DragEvent, ChangeEvent, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  UploadCloud,
  FolderGit2,
  FileCode,
  Globe,
  Cpu,
  Layers,
  X,
  Plus,
  Trash2,
  Sparkles,
  CheckCircle2,
  AlertCircle,
  ExternalLink
} from 'lucide-react'
import { DeploymentItem, DeploymentResult } from './types'

interface DeployWizardModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: (deployment: DeploymentItem) => void
  baseDomain?: string
}

interface EnvVarRow {
  key: string
  value: string
}

export function DeployWizardModal({
  isOpen,
  onClose,
  onSuccess,
  baseDomain = 'cuan.dojo.cc'
}: DeployWizardModalProps) {
  const [intakeTab, setIntakeTab] = useState<'zip' | 'git'>('zip')

  // Common metadata
  const [projectName, setProjectName] = useState('')
  const [slug, setSlug] = useState('')
  const [runtime, setRuntime] = useState<string>('')
  const [envVars, setEnvVars] = useState<EnvVarRow[]>([])

  // ZIP state
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<number>(0)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Git state
  const [repoURL, setRepoURL] = useState('')
  const [branch, setBranch] = useState('main')

  // State & errors
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  if (!isOpen) return null

  const slugify = (text: string) => {
    return text
      .toLowerCase()
      .trim()
      .replace(/[^\w\s-]/g, '')
      .replace(/[\s_-]+/g, '-')
      .replace(/^-+|-+$/g, '')
  }

  const handleProjectNameChange = (e: ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setProjectName(val)
    if (!slug || slug === slugify(projectName)) {
      setSlug(slugify(val))
    }
  }

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }

  const handleDragLeave = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const file = e.dataTransfer.files[0]
      validateAndSetFile(file)
    }
  }

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const file = e.target.files[0]
      validateAndSetFile(file)
    }
  }

  const validateAndSetFile = (file: File) => {
    setErrorMessage(null)
    if (!file.name.toLowerCase().endsWith('.zip')) {
      setErrorMessage('Harap unggah berkas berekstensi .zip yang valid.')
      return
    }
    setSelectedFile(file)

    // Pre-fill project name if empty
    if (!projectName) {
      const baseName = file.name.replace(/\.zip$/i, '')
      setProjectName(baseName)
      setSlug(slugify(baseName))
    }
  }

  const addEnvVar = () => {
    setEnvVars([...envVars, { key: '', value: '' }])
  }

  const removeEnvVar = (index: number) => {
    setEnvVars(envVars.filter((_, i) => i !== index))
  }

  const updateEnvVar = (index: number, field: 'key' | 'value', value: string) => {
    const updated = [...envVars]
    updated[index][field] = value
    setEnvVars(updated)
  }

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)

    if (!projectName.trim()) {
      setErrorMessage('Nama proyek wajib diisi.')
      return
    }

    // Build env_vars map
    const envMap: Record<string, string> = {}
    envVars.forEach((row) => {
      const k = row.key.trim()
      if (k) {
        envMap[k] = row.value
      }
    })

    setIsSubmitting(true)

    if (intakeTab === 'zip') {
      if (!selectedFile) {
        setErrorMessage('Silakan pilih berkas ZIP untuk diunggah.')
        setIsSubmitting(false)
        return
      }

      const formData = new FormData()
      formData.append('file', selectedFile)
      formData.append('project_name', projectName.trim())
      if (slug.trim()) formData.append('slug', slug.trim())
      if (runtime.trim()) formData.append('runtime', runtime.trim())
      if (Object.keys(envMap).length > 0) {
        formData.append('env_vars', JSON.stringify(envMap))
      }

      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/api/deployments/upload')

      xhr.upload.onprogress = (evt) => {
        if (evt.lengthComputable) {
          const percent = Math.round((evt.loaded / evt.total) * 100)
          setUploadProgress(percent)
        }
      }

      xhr.onload = () => {
        setIsSubmitting(false)
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            const data: DeploymentResult = JSON.parse(xhr.responseText)
            onSuccess(data.deployment)
          } catch {
            setErrorMessage('Gagal memproses respon server.')
          }
        } else {
          try {
            const errData = JSON.parse(xhr.responseText)
            setErrorMessage(errData.error || 'Gagal mengunggah dan men-deploy berkas ZIP.')
          } catch {
            setErrorMessage(`Upload gagal dengan status HTTP ${xhr.status}`)
          }
        }
      }

      xhr.onerror = () => {
        setIsSubmitting(false)
        setErrorMessage('Koneksi terputus saat mengunggah berkas ZIP.')
      }

      xhr.send(formData)
    } else {
      // Git Clone intake
      if (!repoURL.trim()) {
        setErrorMessage('URL repositori Git wajib diisi.')
        setIsSubmitting(false)
        return
      }

      try {
        const res = await fetch('/api/deployments/git', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            project_name: projectName.trim(),
            slug: slug.trim(),
            source_type: 'git',
            source_ref: repoURL.trim(),
            branch: branch.trim() || 'main',
            runtime: runtime.trim(),
            env_vars: envMap
          })
        })

        if (!res.ok) {
          const errData = await res.json().catch(() => ({}))
          throw new Error(errData.error || `Deploy gagal dengan status HTTP ${res.status}`)
        }

        const data: DeploymentResult = await res.json()
        setIsSubmitting(false)
        onSuccess(data.deployment)
      } catch (err: any) {
        setIsSubmitting(false)
        setErrorMessage(err.message || 'Gagal clone dan deploy repositori Git.')
      }
    }
  }

  const liveDomainPreview = `${slug || 'app'}.${baseDomain}`

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto animate-in fade-in duration-200">
      <div className="w-full max-w-2xl bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden my-8">
        {/* Header */}
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">Deploy New Project</h2>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Unggah arsip ZIP atau masukkan Git repository untuk deployment instan.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {errorMessage && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Terjadi Kesalahan</AlertTitle>
              <AlertDescription>{errorMessage}</AlertDescription>
            </Alert>
          )}

          {/* Intake Tabs (ZIP vs Git) */}
          <div className="flex rounded-xl bg-slate-100 dark:bg-slate-800 p-1">
            <button
              type="button"
              onClick={() => setIntakeTab('zip')}
              className={`flex-1 flex items-center justify-center gap-2 py-2.5 px-4 rounded-lg text-sm font-semibold transition-all ${
                intakeTab === 'zip'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <UploadCloud className="h-4 w-4 text-emerald-500" />
              Upload ZIP Archive
            </button>
            <button
              type="button"
              onClick={() => setIntakeTab('git')}
              className={`flex-1 flex items-center justify-center gap-2 py-2.5 px-4 rounded-lg text-sm font-semibold transition-all ${
                intakeTab === 'git'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <FolderGit2 className="h-4 w-4 text-blue-500" />
              Git Clone URL
            </button>
          </div>

          {/* TAB 1: ZIP Upload */}
          {intakeTab === 'zip' && (
            <div className="space-y-3">
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={`border-2 border-dashed rounded-xl p-8 text-center cursor-pointer transition-all ${
                  isDragging
                    ? 'border-emerald-500 bg-emerald-50/50 dark:bg-emerald-950/20'
                    : selectedFile
                    ? 'border-emerald-400/80 bg-slate-50 dark:bg-slate-800/40'
                    : 'border-slate-300 dark:border-slate-700 hover:border-emerald-400 hover:bg-slate-50/60 dark:hover:bg-slate-800/20'
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".zip"
                  onChange={handleFileChange}
                  className="hidden"
                />

                {selectedFile ? (
                  <div className="space-y-2">
                    <div className="h-12 w-12 rounded-xl bg-emerald-100 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mx-auto">
                      <CheckCircle2 className="h-6 w-6" />
                    </div>
                    <p className="font-semibold text-slate-800 dark:text-slate-100">{selectedFile.name}</p>
                    <p className="text-xs text-slate-500 dark:text-slate-400">
                      Ukuran: {formatFileSize(selectedFile.size)} • Klik untuk ganti berkas
                    </p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div className="h-12 w-12 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-400 flex items-center justify-center mx-auto">
                      <UploadCloud className="h-6 w-6 text-slate-500 dark:text-slate-400" />
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                        Drag and drop file ZIP ke sini, atau{' '}
                        <span className="text-emerald-600 dark:text-emerald-400 underline">pilih berkas</span>
                      </p>
                      <p className="text-xs text-slate-400 mt-1">Mendukung berkas Static HTML, PHP, dan Node.js hingga 100MB</p>
                    </div>
                  </div>
                )}
              </div>

              {/* Upload Progress Bar */}
              {isSubmitting && uploadProgress > 0 && (
                <div className="space-y-1.5 pt-1">
                  <div className="flex justify-between text-xs font-medium text-slate-600 dark:text-slate-300">
                    <span>Mengunggah arsip proyek...</span>
                    <span>{uploadProgress}%</span>
                  </div>
                  <div className="h-2 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-emerald-500 to-teal-400 transition-all duration-300"
                      style={{ width: `${uploadProgress}%` }}
                    />
                  </div>
                </div>
              )}
            </div>
          )}

          {/* TAB 2: Git Clone */}
          {intakeTab === 'git' && (
            <div className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  Git Repository URL <span className="text-red-500">*</span>
                </label>
                <Input
                  type="url"
                  placeholder="https://github.com/organization/repository.git"
                  value={repoURL}
                  onChange={(e) => {
                    const val = e.target.value
                    setRepoURL(val)
                    if (!projectName && val) {
                      const match = val.match(/\/([^/]+?)(\.git)?$/)
                      if (match && match[1]) {
                        setProjectName(match[1])
                        setSlug(slugify(match[1]))
                      }
                    }
                  }}
                  className="font-mono text-sm"
                  required
                />
                <p className="text-[11px] text-slate-400">
                  Repositori publik GitHub, GitLab, atau Git server mandiri.
                </p>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  Branch
                </label>
                <Input
                  type="text"
                  placeholder="main"
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>
            </div>
          )}

          {/* Metadata Section: Name & Slug */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2 border-t border-slate-100 dark:border-slate-800">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                Nama Proyek <span className="text-red-500">*</span>
              </label>
              <Input
                type="text"
                placeholder="Contoh: Toko Online Saya"
                value={projectName}
                onChange={handleProjectNameChange}
                required
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                Subdomain Slug
              </label>
              <Input
                type="text"
                placeholder="toko-online"
                value={slug}
                onChange={(e) => setSlug(slugify(e.target.value))}
                className="font-mono text-sm"
              />
            </div>
          </div>

          {/* Subdomain Preview */}
          <div className="p-3 bg-slate-50 dark:bg-slate-800/60 rounded-xl border border-slate-200 dark:border-slate-800 flex items-center justify-between text-xs">
            <span className="text-slate-500 dark:text-slate-400">Subdomain Live Preview:</span>
            <span className="font-mono font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-1">
              https://{liveDomainPreview}
              <ExternalLink className="h-3 w-3" />
            </span>
          </div>

          {/* Stack Auto-Detection & Override */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
              <Layers className="h-3.5 w-3.5 text-slate-400" />
              Runtime Stack Profile
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {[
                { id: '', label: 'Auto Detect', icon: Sparkles, desc: 'Otomatis' },
                { id: 'static', label: 'Static HTML', icon: Globe, desc: 'index.html' },
                { id: 'php', label: 'PHP / WP', icon: FileCode, desc: 'index.php' },
                { id: 'nodejs', label: 'Node.js', icon: Cpu, desc: 'package.json' }
              ].map((item) => {
                const Icon = item.icon
                const isSelected = runtime === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setRuntime(item.id)}
                    className={`p-3 rounded-xl border text-left transition-all ${
                      isSelected
                        ? 'border-emerald-500 bg-emerald-50/60 dark:bg-emerald-950/30 ring-1 ring-emerald-500'
                        : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'
                    }`}
                  >
                    <Icon className={`h-4 w-4 mb-1.5 ${isSelected ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-400'}`} />
                    <p className="text-xs font-semibold text-slate-900 dark:text-white">{item.label}</p>
                    <p className="text-[10px] text-slate-400">{item.desc}</p>
                  </button>
                )
              })}
            </div>
          </div>

          {/* Environment Variables Key-Value Editor */}
          <div className="space-y-2 pt-2 border-t border-slate-100 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                Environment Variables (Opsional)
              </label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addEnvVar}
                className="h-7 text-xs flex items-center gap-1"
              >
                <Plus className="h-3 w-3" /> Tambah Env
              </Button>
            </div>

            {envVars.length === 0 ? (
              <p className="text-xs text-slate-400 italic">Belum ada variabel lingkungan yang disetel.</p>
            ) : (
              <div className="space-y-2 max-h-40 overflow-y-auto pr-1">
                {envVars.map((row, idx) => (
                  <div key={idx} className="flex items-center gap-2">
                    <Input
                      placeholder="KEY (e.g. PORT)"
                      value={row.key}
                      onChange={(e) => updateEnvVar(idx, 'key', e.target.value)}
                      className="font-mono text-xs uppercase"
                    />
                    <Input
                      placeholder="VALUE"
                      value={row.value}
                      onChange={(e) => updateEnvVar(idx, 'value', e.target.value)}
                      className="font-mono text-xs"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => removeEnvVar(idx)}
                      className="h-9 px-2 text-red-500 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950/30"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Footer Actions */}
          <div className="pt-4 border-t border-slate-100 dark:border-slate-800 flex items-center justify-end gap-3">
            <Button type="button" variant="outline" onClick={onClose} disabled={isSubmitting}>
              Batal
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || (intakeTab === 'zip' && !selectedFile)}
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold"
            >
              {isSubmitting ? 'Memproses Deployment...' : 'Deploy Sekarang'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
