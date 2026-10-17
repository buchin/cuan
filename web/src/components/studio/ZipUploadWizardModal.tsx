import { useState, useRef, DragEvent, ChangeEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  UploadCloud,
  FileArchive,
  CheckCircle2,
  AlertCircle,
  X,
  Sparkles,
  Layers,
  Database,
  KeyRound,
  ListTodo,
  ArrowRight,
  RefreshCw,
  Cpu
} from 'lucide-react'
import { CreateFromZipResponse, InspectionResultDTO } from './types'

interface ZipUploadWizardModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: (result: CreateFromZipResponse) => void
  onProjectCreated?: (result: CreateFromZipResponse) => void
}

export function ZipUploadWizardModal({
  isOpen,
  onClose,
  onSuccess,
  onProjectCreated,
}: ZipUploadWizardModalProps) {
  const [file, setFile] = useState<File | null>(null)
  const [projectName, setProjectName] = useState('')
  const [isDragging, setIsDragging] = useState(false)
  const [step, setStep] = useState<'upload' | 'inspecting' | 'reviewed' | 'creating'>('upload')
  const [uploadProgress, setUploadProgress] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [inspection, setInspection] = useState<InspectionResultDTO | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  if (!isOpen) return null

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(true)
  }

  const handleDragLeave = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFileSelected(e.dataTransfer.files[0])
    }
  }

  const handleFileInputChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      handleFileSelected(e.target.files[0])
    }
  }

  const handleFileSelected = (selectedFile: File) => {
    setError(null)
    if (!selectedFile.name.toLowerCase().endsWith('.zip')) {
      setError('Hanya file arsip .zip yang didukung.')
      return
    }
    if (selectedFile.size > 150 * 1024 * 1024) {
      setError('Ukuran file melebihi batas kuota ekstraksi (maks 150 MB).')
      return
    }
    setFile(selectedFile)
    if (!projectName) {
      const cleanName = selectedFile.name
        .replace(/\.zip$/i, '')
        .replace(/[-_]/g, ' ')
        .replace(/\b\w/g, (c) => c.toUpperCase())
      setProjectName(cleanName)
    }
  }

  const handleInspect = async () => {
    if (!file) return
    setError(null)
    setStep('inspecting')
    setUploadProgress(20)

    const timer = setInterval(() => {
      setUploadProgress((prev) => (prev < 85 ? prev + 15 : prev))
    }, 250)

    try {
      const formData = new FormData()
      formData.append('file', file)

      const res = await fetch('/api/v1/inspector/inspect-zip', {
        method: 'POST',
        body: formData,
      })

      clearInterval(timer)
      setUploadProgress(100)

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || data.message || `HTTP ${res.status}: Gagal memindai arsip ZIP`)
      }

      setInspection(data.inspection || data)
      setStep('reviewed')
    } catch (err: any) {
      clearInterval(timer)
      setError(err.message || 'Gagal memindai arsip ZIP. Pastikan file valid.')
      setStep('upload')
    }
  }

  const handleCreateProject = async () => {
    if (!file) return
    setError(null)
    setStep('creating')
    setUploadProgress(30)

    const timer = setInterval(() => {
      setUploadProgress((prev) => (prev < 90 ? prev + 10 : prev))
    }, 200)

    try {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('name', projectName.trim() || file.name.replace(/\.zip$/i, ''))

      const res = await fetch('/api/v1/projects/from-zip', {
        method: 'POST',
        body: formData,
      })

      clearInterval(timer)
      setUploadProgress(100)

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || data.message || `HTTP ${res.status}: Gagal membuat proyek dari ZIP`)
      }

      if (onSuccess) onSuccess(data)
      if (onProjectCreated) onProjectCreated(data)
      onClose()
    } catch (err: any) {
      clearInterval(timer)
      setError(err.message || 'Terjadi kesalahan saat mengekstrak dan membuat project.')
      setStep(inspection ? 'reviewed' : 'upload')
    }
  }

  const resetUpload = () => {
    setFile(null)
    setInspection(null)
    setStep('upload')
    setUploadProgress(0)
    setError(null)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl w-full max-w-3xl shadow-2xl overflow-hidden flex flex-col max-h-[92vh]">
        {/* Header */}
        <div className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-gradient-to-r from-emerald-500/10 via-teal-500/5 to-slate-50 dark:to-slate-900">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-2xl bg-emerald-500 text-slate-950 font-bold flex items-center justify-center shadow-lg shadow-emerald-500/20">
              <UploadCloud className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-black text-slate-900 dark:text-white tracking-tight">
                  New Project from ZIP
                </h2>
                <Badge className="bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 text-[10px] font-mono py-0 px-2">
                  AI Script Onboarding
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Unggah file ZIP script (CodeCanyon/GitHub). AI Inspector membedah stack, konfigurasi .env, dan skema database otomatis.
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Error Alert */}
        {error && (
          <div className="px-6 pt-4">
            <Alert className="bg-red-500/10 border-red-500/20 text-red-700 dark:text-red-300">
              <AlertCircle className="h-4 w-4 text-red-500" />
              <AlertTitle className="text-xs font-bold">Gagal Memproses ZIP</AlertTitle>
              <AlertDescription className="text-xs mt-0.5">{error}</AlertDescription>
            </Alert>
          </div>
        )}

        {/* Content */}
        <div className="p-6 overflow-y-auto space-y-5 flex-1">
          {step === 'upload' && (
            <div className="space-y-4">
              {/* Dropzone */}
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={`border-2 border-dashed rounded-3xl p-8 sm:p-12 text-center transition-all cursor-pointer flex flex-col items-center justify-center gap-3.5 ${
                  isDragging
                    ? 'border-emerald-500 bg-emerald-500/5 scale-[0.99]'
                    : file
                    ? 'border-emerald-500/60 bg-emerald-500/5'
                    : 'border-slate-300 dark:border-slate-700 hover:border-emerald-500/50 bg-slate-50/50 dark:bg-slate-950/50'
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".zip"
                  onChange={handleFileInputChange}
                  className="hidden"
                />

                {file ? (
                  <>
                    <div className="h-16 w-16 rounded-2xl bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shadow-md">
                      <FileArchive className="h-8 w-8" />
                    </div>
                    <div>
                      <h4 className="text-sm font-bold text-slate-900 dark:text-white">
                        {file.name}
                      </h4>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5 font-mono">
                        {(file.size / (1024 * 1024)).toFixed(2)} MB &bull; Klik atau drag file lain untuk mengganti
                      </p>
                    </div>
                    <Badge className="bg-emerald-500 text-slate-950 font-bold text-xs py-0.5 px-3">
                      File Siap Dianalisis
                    </Badge>
                  </>
                ) : (
                  <>
                    <div className="h-16 w-16 rounded-2xl bg-slate-100 dark:bg-slate-800 text-slate-400 flex items-center justify-center">
                      <UploadCloud className="h-8 w-8 text-emerald-500" />
                    </div>
                    <div>
                      <h4 className="text-sm font-bold text-slate-800 dark:text-slate-200">
                        Drag & Drop File ZIP Script ke Sini
                      </h4>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                        atau klik untuk memilih dari komputer (Maksimal 150 MB, proteksi isolasi sandbox zip-slip)
                      </p>
                    </div>
                    <div className="flex items-center gap-2 text-[11px] text-slate-400 font-mono">
                      <span>Mendukung: Laravel, Node.js, Next.js, Django, WordPress, dsb.</span>
                    </div>
                  </>
                )}
              </div>

              {/* Project Name Field */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Nama Project Baru
                </label>
                <Input
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  placeholder="Contoh: Toko Online Modern Laravel"
                  className="text-xs"
                />
              </div>
            </div>
          )}

          {(step === 'inspecting' || step === 'creating') && (
            <div className="py-12 px-6 text-center space-y-4">
              <div className="relative w-16 h-16 mx-auto">
                <RefreshCw className="h-16 w-16 text-emerald-500 animate-spin" />
                <Sparkles className="h-6 w-6 text-emerald-400 absolute inset-0 m-auto" />
              </div>
              <div className="space-y-1">
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  {step === 'inspecting'
                    ? 'AI Inspector Sedang Membedah ZIP...'
                    : 'Mengekstrak & Membuat Workspace Modular...'}
                </h3>
                <p className="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto">
                  {step === 'inspecting'
                    ? 'Memeriksa signature file stack, mengekstrak variabel environment .env, dan mendeteksi skema database .sql.'
                    : 'Mengisolasi workspace, mengaktifkan modul dinamis, dan menyusun task setup pipeline di Kanban board.'}
                </p>
              </div>

              {/* Progress bar */}
              <div className="w-full max-w-md mx-auto bg-slate-100 dark:bg-slate-800 rounded-full h-2.5 overflow-hidden">
                <div
                  className="bg-emerald-500 h-2.5 rounded-full transition-all duration-300"
                  style={{ width: `${uploadProgress}%` }}
                />
              </div>
              <span className="text-xs font-mono text-slate-400">{uploadProgress}% Selesai</span>
            </div>
          )}

          {step === 'reviewed' && inspection && (
            <div className="space-y-4">
              {/* Stack Inspector Result Card */}
              <div className="p-4 rounded-2xl bg-gradient-to-r from-emerald-500/10 via-slate-50 to-slate-100 dark:from-emerald-950/30 dark:via-slate-900/60 dark:to-slate-900 border border-emerald-500/30 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="p-1.5 rounded-xl bg-emerald-500/20 text-emerald-600 dark:text-emerald-400">
                      <Cpu className="h-4 w-4" />
                    </span>
                    <div>
                      <h4 className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                        Hasil Analisis Stack Inspector
                      </h4>
                      <p className="text-[11px] text-slate-500 dark:text-slate-400">
                        File: {inspection.archive_name || file?.name} &bull; Total: {inspection.total_files} file ({(inspection.total_size / (1024 * 1024)).toFixed(2)} MB)
                      </p>
                    </div>
                  </div>

                  <Badge className="bg-emerald-500 text-slate-950 font-bold text-xs capitalize">
                    {inspection.tech_stack?.framework || inspection.tech_stack?.primary_stack} ({(inspection.tech_stack?.confidence * 100).toFixed(0)}% Match)
                  </Badge>
                </div>

                {/* Signatures */}
                {inspection.tech_stack?.signatures && inspection.tech_stack.signatures.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 items-center pt-1 text-xs">
                    <span className="text-[10px] text-slate-400">Signature terdeteksi:</span>
                    {inspection.tech_stack.signatures.map((sig, idx) => (
                      <span key={idx} className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300">
                        {sig}
                      </span>
                    ))}
                  </div>
                )}
              </div>

              {/* Grid: Env & SQL Schemas */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                {/* Env card */}
                <div className="p-3.5 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                      <KeyRound className="h-3.5 w-3.5 text-amber-500" />
                      <span>Deteksi Konfigurasi .env</span>
                    </span>
                    <Badge variant="outline" className="text-[10px] font-mono">
                      {inspection.env?.all_keys?.length || 0} Variabel
                    </Badge>
                  </div>
                  <p className="text-[11px] text-slate-500 dark:text-slate-400">
                    {inspection.env?.source_file ? `Ditemukan di ${inspection.env.source_file}` : 'Variabel environment siap dikonfigurasi.'}
                  </p>
                  {inspection.env?.required_keys && inspection.env.required_keys.length > 0 && (
                    <div className="flex flex-wrap gap-1 pt-1">
                      {inspection.env.required_keys.slice(0, 4).map((k, i) => (
                        <span key={i} className="text-[9px] font-mono px-1 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                          {k}
                        </span>
                      ))}
                      {inspection.env.required_keys.length > 4 && (
                        <span className="text-[9px] text-slate-400">+{inspection.env.required_keys.length - 4} lainnya</span>
                      )}
                    </div>
                  )}
                </div>

                {/* SQL card */}
                <div className="p-3.5 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                      <Database className="h-3.5 w-3.5 text-blue-500" />
                      <span>Deteksi Skema Database .sql</span>
                    </span>
                    <Badge variant="outline" className="text-[10px] font-mono">
                      {inspection.sql_files?.length || 0} File SQL
                    </Badge>
                  </div>
                  <p className="text-[11px] text-slate-500 dark:text-slate-400">
                    {inspection.sql_files && inspection.sql_files.length > 0
                      ? inspection.sql_files.map((s) => s.path).join(', ')
                      : 'Tidak ditemukan file .sql mentah (menggunakan migrasi framework).'}
                  </p>
                </div>
              </div>

              {/* Recommended Modules */}
              <div className="p-3.5 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                    <Layers className="h-3.5 w-3.5 text-indigo-500" />
                    <span>Modul Workspace yang Diaktifkan</span>
                  </span>
                  <span className="text-[10px] text-slate-400">Dapat diubah kapan saja via "+ Add Modules"</span>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {(inspection.recommended_modules || ['tasks', 'discussions', 'servers', 'repositories']).map((mod, i) => (
                    <span key={i} className="text-[11px] font-medium px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 capitalize flex items-center gap-1">
                      <CheckCircle2 className="h-3 w-3" />
                      {mod}
                    </span>
                  ))}
                </div>
              </div>

              {/* Auto-Generated Tasks Preview */}
              {inspection.recommended_tasks && inspection.recommended_tasks.length > 0 && (
                <div className="space-y-2">
                  <h4 className="text-xs font-bold text-slate-700 dark:text-slate-300 flex items-center gap-1.5 uppercase tracking-wider">
                    <ListTodo className="h-3.5 w-3.5 text-emerald-500" />
                    <span>Auto-Generated Setup Pipeline ({inspection.recommended_tasks.length} Tasks)</span>
                  </h4>
                  <div className="p-3 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-2 max-h-44 overflow-y-auto">
                    {inspection.recommended_tasks.map((t, idx) => (
                      <div key={idx} className="flex items-start justify-between gap-2 p-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
                        <div className="space-y-0.5">
                          <span className="text-xs font-bold text-slate-900 dark:text-white block">
                            {idx + 1}. {t.title}
                          </span>
                          <span className="text-[11px] text-slate-400 block line-clamp-1">
                            {t.description}
                          </span>
                        </div>
                        <Badge className="text-[9px] uppercase font-mono py-0 px-1.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400">
                          {t.priority}
                        </Badge>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 flex items-center justify-between">
          <div>
            {step === 'reviewed' ? (
              <Button variant="ghost" size="sm" onClick={resetUpload} className="text-xs">
                Upload File Lain
              </Button>
            ) : (
              <span className="text-[11px] text-slate-400">Format: ZIP &bull; Limit: 150MB</span>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={onClose} disabled={step === 'inspecting' || step === 'creating'}>
              Batal
            </Button>

            {step === 'upload' && (
              <Button
                size="sm"
                onClick={handleInspect}
                disabled={!file}
                className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold gap-1.5"
              >
                <Sparkles className="h-4 w-4" />
                <span>Analisis Arsip ZIP</span>
              </Button>
            )}

            {step === 'reviewed' && (
              <Button
                size="sm"
                onClick={handleCreateProject}
                className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold gap-1.5 shadow-md shadow-emerald-600/20"
              >
                <span>Buat Proyek & Setup Pipeline</span>
                <ArrowRight className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
