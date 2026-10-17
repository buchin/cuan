import { useState, useEffect } from 'react'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Badge } from '../../ui/badge'
import {
  Code,
  UploadCloud,
  FileArchive,
  History,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  Plus,
  Trash2,
  ChevronDown,
  ChevronRight,
  Sliders
} from 'lucide-react'
import {
  WordPressAssetDTO,
  WordPressScriptDTO,
  WordPressConfigTweaks,
  WordPressWebmasterActionDTO
} from '../types'

export interface WordPressWebmasterPanelProps {
  projectId: string
  projectName?: string
  deploymentId?: string
  onToast?: (type: 'success' | 'error', message: string) => void
  className?: string
}

export function WordPressWebmasterPanel({
  projectId,
  projectName: _projectName = 'WordPress Project',
  deploymentId: _deploymentId,
  onToast,
  className = '',
}: WordPressWebmasterPanelProps) {
  // Active sub-tab in Webmaster Studio
  const [activeTab, setActiveTab] = useState<'assets' | 'scripts' | 'config' | 'actions'>('assets')

  // Feedback notifications
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // ---------------------------------------------------------------------------
  // Tab 1: Theme & Plugin Asset Dropzone State
  // ---------------------------------------------------------------------------
  const [assets, setAssets] = useState<WordPressAssetDTO[]>([])
  const [loadingAssets, setLoadingAssets] = useState(false)
  const [uploadingAsset, setUploadingAsset] = useState(false)
  const [assetDragOver, setAssetDragOver] = useState(false)
  const [assetFile, setAssetFile] = useState<File | null>(null)
  const [assetKind, setAssetKind] = useState<'theme' | 'plugin'>('theme')
  const [assetSlug, setAssetSlug] = useState('')
  const [assetAutoActivate, setAssetAutoActivate] = useState(true)

  // ---------------------------------------------------------------------------
  // Tab 2: Tracking Script Injector State
  // ---------------------------------------------------------------------------
  const [scripts, setScripts] = useState<WordPressScriptDTO[]>([])
  const [loadingScripts, setLoadingScripts] = useState(false)
  const [injectingScript, setInjectingScript] = useState(false)
  const [scriptName, setScriptName] = useState('')
  const [scriptType, setScriptType] = useState('custom')
  const [scriptPosition, setScriptPosition] = useState<'head' | 'footer'>('head')
  const [scriptContent, setScriptContent] = useState('')
  const [scriptEnabled, setScriptEnabled] = useState(true)
  const [expandedScriptId, setExpandedScriptId] = useState<string | null>(null)

  // ---------------------------------------------------------------------------
  // Tab 3: Configuration Tweaker (wp-config.php)
  // ---------------------------------------------------------------------------
  const [loadingConfig, setLoadingConfig] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)
  const [wpDebug, setWpDebug] = useState(false)
  const [wpDebugLog, setWpDebugLog] = useState(true)
  const [wpDebugDisplay, setWpDebugDisplay] = useState(false)
  const [scriptDebug, setScriptDebug] = useState(false)
  const [wpMemoryLimit, setWpMemoryLimit] = useState('256M')
  const [wpMaxMemoryLimit, setWpMaxMemoryLimit] = useState('512M')

  // ---------------------------------------------------------------------------
  // Tab 4: Webmaster Audit Action History
  // ---------------------------------------------------------------------------
  const [actions, setActions] = useState<WordPressWebmasterActionDTO[]>([])
  const [loadingActions, setLoadingActions] = useState(false)
  const [filterActionType, setFilterActionType] = useState('all')
  const [expandedActionId, setExpandedActionId] = useState<string | null>(null)

  // Auto-clear messages
  useEffect(() => {
    if (errorMsg || successMsg) {
      const timer = setTimeout(() => {
        setErrorMsg(null)
        setSuccessMsg(null)
      }, 5000)
      return () => clearTimeout(timer)
    }
  }, [errorMsg, successMsg])

  // ===========================================================================
  // Fetch Functions
  // ===========================================================================

  const fetchAssets = async () => {
    if (!projectId) return
    setLoadingAssets(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/assets`)
      if (res.ok) {
        const data = await res.json()
        setAssets(Array.isArray(data) ? data : [])
      }
    } catch {
      // Ignore network polling error
    } finally {
      setLoadingAssets(false)
    }
  }

  const fetchScripts = async () => {
    if (!projectId) return
    setLoadingScripts(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/scripts`)
      if (res.ok) {
        const data = await res.json()
        setScripts(Array.isArray(data) ? data : [])
      }
    } catch {
      // Ignore network polling error
    } finally {
      setLoadingScripts(false)
    }
  }

  const fetchConfig = async () => {
    if (!projectId) return
    setLoadingConfig(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/config`)
      if (res.ok) {
        const data: WordPressConfigTweaks = await res.json()
        const t = data.tweaks || data.constants || {}
        if (t.WP_DEBUG !== undefined) setWpDebug(Boolean(t.WP_DEBUG))
        if (t.WP_DEBUG_LOG !== undefined) setWpDebugLog(Boolean(t.WP_DEBUG_LOG))
        if (t.WP_DEBUG_DISPLAY !== undefined) setWpDebugDisplay(Boolean(t.WP_DEBUG_DISPLAY))
        if (t.SCRIPT_DEBUG !== undefined) setScriptDebug(Boolean(t.SCRIPT_DEBUG))
        if (t.WP_MEMORY_LIMIT) setWpMemoryLimit(String(t.WP_MEMORY_LIMIT))
        if (t.WP_MAX_MEMORY_LIMIT) setWpMaxMemoryLimit(String(t.WP_MAX_MEMORY_LIMIT))
      }
    } catch {
      // Ignore network error
    } finally {
      setLoadingConfig(false)
    }
  }

  const fetchActions = async () => {
    if (!projectId) return
    setLoadingActions(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/actions`)
      if (res.ok) {
        const data = await res.json()
        setActions(Array.isArray(data) ? data : [])
      }
    } catch {
      // Ignore network error
    } finally {
      setLoadingActions(false)
    }
  }

  // Load active tab data
  useEffect(() => {
    if (activeTab === 'assets') fetchAssets()
    else if (activeTab === 'scripts') fetchScripts()
    else if (activeTab === 'config') fetchConfig()
    else if (activeTab === 'actions') fetchActions()
  }, [projectId, activeTab])

  // ===========================================================================
  // Tab 1: Asset Handlers
  // ===========================================================================

  const handleAssetUpload = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    if (!assetFile) {
      setErrorMsg('Pilih berkas ZIP theme atau plugin terlebih dahulu.')
      return
    }

    setUploadingAsset(true)
    setErrorMsg(null)
    setSuccessMsg(null)

    try {
      const formData = new FormData()
      formData.append('file', assetFile)
      formData.append('kind', assetKind)
      formData.append('slug', assetSlug.trim())
      formData.append('activate', assetAutoActivate ? 'true' : 'false')

      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/assets/upload`, {
        method: 'POST',
        body: formData,
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal mengunggah asset ZIP')
      }

      const installed = await res.json()
      setSuccessMsg(`✅ Asset '${installed.name || installed.slug}' berhasil diunggah dan diinstal!`)
      onToast?.('success', `Asset '${installed.slug}' terinstal.`)
      setAssetFile(null)
      setAssetSlug('')
      fetchAssets()
    } catch (err: any) {
      setErrorMsg(err.message || 'Terjadi kesalahan saat upload.')
      onToast?.('error', err.message || 'Upload gagal.')
    } finally {
      setUploadingAsset(false)
    }
  }

  const handleToggleAsset = async (assetId: string, currentActive: boolean) => {
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/assets/${assetId}/status`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ is_active: !currentActive }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal mengubah status asset')
      }
      setSuccessMsg(`Status asset berhasil diubah.`)
      fetchAssets()
    } catch (err: any) {
      setErrorMsg(err.message)
    }
  }

  const handleDeleteAsset = async (assetId: string) => {
    if (!confirm('Apakah Anda yakin ingin menghapus asset ini?')) return
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/assets/${assetId}`, {
        method: 'DELETE',
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal menghapus asset')
      }
      setSuccessMsg('Asset berhasil dihapus.')
      fetchAssets()
    } catch (err: any) {
      setErrorMsg(err.message)
    }
  }

  // ===========================================================================
  // Tab 2: Script Injector Handlers
  // ===========================================================================

  const applyScriptPreset = (preset: string) => {
    setScriptType(preset)
    if (preset === 'gtm') {
      setScriptName('Google Tag Manager')
      setScriptPosition('head')
      setScriptContent(`<!-- Google Tag Manager -->
<script>(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','GTM-XXXXXXX');</script>
<!-- End Google Tag Manager -->`)
    } else if (preset === 'ga4') {
      setScriptName('Google Analytics 4')
      setScriptPosition('head')
      setScriptContent(`<!-- Google tag (gtag.js) -->
<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXXXXXXXX"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'G-XXXXXXXXXX');
</script>`)
    } else if (preset === 'meta') {
      setScriptName('Meta Pixel (Facebook)')
      setScriptPosition('head')
      setScriptContent(`<!-- Meta Pixel Code -->
<script>
!function(f,b,e,v,n,t,s)
{if(f.fbq)return;n=f.fbq=function(){n.callMethod?
n.callMethod.apply(n,arguments):n.queue.push(arguments)};
if(!f._fbq)f._fbq=n;n.push=n;n.loaded=!0;n.version='2.0';
n.queue=[];t=b.createElement(e);t.async=!0;
t.src=v;s=b.getElementsByTagName(e)[0];
s.parentNode.insertBefore(t,s)}(window, document,'script',
'https://connect.facebook.net/en_US/fbevents.js');
fbq('init', 'YOUR_PIXEL_ID');
fbq('track', 'PageView');
</script>
<!-- End Meta Pixel Code -->`)
    } else {
      setScriptName('Custom Header Tracker')
      setScriptPosition('head')
      setScriptContent('<script>\n  // Custom tracking logic\n</script>')
    }
  }

  const handleInjectScript = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!scriptName.trim() || !scriptContent.trim()) {
      setErrorMsg('Nama script dan konten script wajib diisi.')
      return
    }

    setInjectingScript(true)
    setErrorMsg(null)
    setSuccessMsg(null)

    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/scripts`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: scriptName.trim(),
          type: scriptType,
          position: scriptPosition,
          content: scriptContent,
          enabled: scriptEnabled,
        }),
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal menginjeksi script')
      }

      setSuccessMsg(`Script '${scriptName}' berhasil diinjeksi via mu-plugins!`)
      onToast?.('success', `Script tracking terinjeksi.`)
      setScriptName('')
      setScriptContent('')
      fetchScripts()
    } catch (err: any) {
      setErrorMsg(err.message)
      onToast?.('error', err.message)
    } finally {
      setInjectingScript(false)
    }
  }

  const handleToggleScript = async (scriptId: string, currentEnabled: boolean) => {
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/scripts/${scriptId}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !currentEnabled }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal mengubah status script')
      }
      setSuccessMsg('Status script diperbarui.')
      fetchScripts()
    } catch (err: any) {
      setErrorMsg(err.message)
    }
  }

  const handleDeleteScript = async (scriptId: string) => {
    if (!confirm('Hapus script tracking ini dari WordPress?')) return
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/scripts/${scriptId}`, {
        method: 'DELETE',
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal menghapus script')
      }
      setSuccessMsg('Script tracking berhasil dihapus.')
      fetchScripts()
    } catch (err: any) {
      setErrorMsg(err.message)
    }
  }

  // ===========================================================================
  // Tab 3: Config Tweaker Handlers
  // ===========================================================================

  const handleSaveConfig = async () => {
    setSavingConfig(true)
    setErrorMsg(null)
    setSuccessMsg(null)

    const tweaks = {
      WP_DEBUG: wpDebug,
      WP_DEBUG_LOG: wpDebugLog,
      WP_DEBUG_DISPLAY: wpDebugDisplay,
      SCRIPT_DEBUG: scriptDebug,
      WP_MEMORY_LIMIT: wpMemoryLimit,
      WP_MAX_MEMORY_LIMIT: wpMaxMemoryLimit,
    }

    try {
      const res = await fetch(`/api/v1/projects/${projectId}/wordpress/config`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tweaks }),
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal menyimpan konfigurasi wp-config.php')
      }

      setSuccessMsg('⚡ Konfigurasi wp-config.php berhasil di-tweak dan lolos verifikasi sintaks PHP!')
      onToast?.('success', 'Konfigurasi wp-config.php diperbarui.')
      fetchConfig()
    } catch (err: any) {
      setErrorMsg(err.message)
      onToast?.('error', err.message)
    } finally {
      setSavingConfig(false)
    }
  }

  // Filter actions
  const filteredActions = actions.filter((act) => {
    if (filterActionType === 'all') return true
    return act.action_type === filterActionType
  })

  return (
    <div className={`space-y-4 ${className}`}>
      {/* Notifications */}
      {errorMsg && (
        <div className="flex items-start gap-2.5 p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 text-xs animate-in fade-in">
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
          <div className="flex-1">{errorMsg}</div>
        </div>
      )}

      {successMsg && (
        <div className="flex items-start gap-2.5 p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs animate-in fade-in">
          <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5" />
          <div className="flex-1">{successMsg}</div>
        </div>
      )}

      {/* Sub-tab Navigation */}
      <div className="flex items-center gap-1.5 p-1 rounded-xl bg-slate-100 dark:bg-slate-800/80 border border-slate-200 dark:border-slate-700/60 text-xs font-medium">
        <button
          onClick={() => setActiveTab('assets')}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all ${
            activeTab === 'assets'
              ? 'bg-white dark:bg-slate-900 text-blue-600 dark:text-blue-400 shadow-2xs font-semibold'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          }`}
        >
          <FileArchive className="h-3.5 w-3.5" />
          <span>Themes & Plugins ({assets.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('scripts')}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all ${
            activeTab === 'scripts'
              ? 'bg-white dark:bg-slate-900 text-blue-600 dark:text-blue-400 shadow-2xs font-semibold'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          }`}
        >
          <Code className="h-3.5 w-3.5" />
          <span>Script Injector ({scripts.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('config')}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all ${
            activeTab === 'config'
              ? 'bg-white dark:bg-slate-900 text-blue-600 dark:text-blue-400 shadow-2xs font-semibold'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          }`}
        >
          <Sliders className="h-3.5 w-3.5" />
          <span>Config Tweaker</span>
        </button>

        <button
          onClick={() => setActiveTab('actions')}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all ${
            activeTab === 'actions'
              ? 'bg-white dark:bg-slate-900 text-blue-600 dark:text-blue-400 shadow-2xs font-semibold'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white'
          }`}
        >
          <History className="h-3.5 w-3.5" />
          <span>Audit Log ({actions.length})</span>
        </button>
      </div>

      {/* ===================================================================== */}
      {/* TAB 1: Theme & Plugin Dropzone & Manager                              */}
      {/* ===================================================================== */}
      {activeTab === 'assets' && (
        <div className="space-y-4">
          {/* Dropzone Card */}
          <div className="p-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-3 shadow-2xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <UploadCloud className="h-4 w-4 text-blue-500" />
                <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  Upload & Ekstraksi ZIP Theme / Plugin
                </h3>
              </div>
              <Badge className="text-[10px] bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20">
                Auto-Activation
              </Badge>
            </div>

            <div
              onDragOver={(e) => {
                e.preventDefault()
                setAssetDragOver(true)
              }}
              onDragLeave={() => setAssetDragOver(false)}
              onDrop={(e) => {
                e.preventDefault()
                setAssetDragOver(false)
                if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                  const f = e.dataTransfer.files[0]
                  if (f.name.toLowerCase().endsWith('.zip')) {
                    setAssetFile(f)
                    if (!assetSlug) {
                      setAssetSlug(f.name.replace(/\.zip$/i, '').toLowerCase().replace(/[^a-z0-9_-]/g, '-'))
                    }
                  } else {
                    setErrorMsg('File harus berupa arsip .ZIP')
                  }
                }
              }}
              className={`border-2 border-dashed rounded-xl p-5 text-center transition-all ${
                assetDragOver
                  ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-950/20'
                  : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'
              }`}
            >
              <div className="max-w-md mx-auto space-y-2">
                <FileArchive className="h-8 w-8 text-blue-500 mx-auto" />
                <p className="text-xs font-medium text-slate-700 dark:text-slate-300">
                  {assetFile ? assetFile.name : 'Tarik & Letakkan file ZIP theme atau plugin di sini'}
                </p>
                <div className="flex items-center justify-center gap-2 pt-1">
                  <label className="cursor-pointer">
                    <Button type="button" variant="outline" size="sm" className="h-7 text-xs pointer-events-none">
                      {assetFile ? 'Ganti File' : 'Pilih File ZIP'}
                    </Button>
                    <input
                      type="file"
                      accept=".zip,application/zip"
                      className="hidden"
                      onChange={(e) => {
                        if (e.target.files && e.target.files.length > 0) {
                          const f = e.target.files[0]
                          setAssetFile(f)
                          if (!assetSlug) {
                            setAssetSlug(f.name.replace(/\.zip$/i, '').toLowerCase().replace(/[^a-z0-9_-]/g, '-'))
                          }
                        }
                      }}
                    />
                  </label>
                  {assetFile && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => setAssetFile(null)}
                      className="h-7 text-xs text-rose-500 hover:text-rose-600"
                    >
                      Batal
                    </Button>
                  )}
                </div>
              </div>
            </div>

            {/* Upload Options Form */}
            {assetFile && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1 text-xs">
                <div>
                  <label className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                    Jenis Asset:
                  </label>
                  <select
                    value={assetKind}
                    onChange={(e) => setAssetKind(e.target.value as any)}
                    className="w-full text-xs px-2.5 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200"
                  >
                    <option value="theme">Theme (wp-content/themes)</option>
                    <option value="plugin">Plugin (wp-content/plugins)</option>
                  </select>
                </div>

                <div>
                  <label className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                    Custom Slug / Folder:
                  </label>
                  <Input
                    type="text"
                    value={assetSlug}
                    onChange={(e) => setAssetSlug(e.target.value)}
                    placeholder="e.g. generatepress"
                    className="text-xs h-8 font-mono"
                  />
                </div>

                <div className="flex items-end gap-2">
                  <div className="flex-1 flex items-center gap-1.5 pb-2">
                    <input
                      type="checkbox"
                      id="autoAct"
                      checked={assetAutoActivate}
                      onChange={(e) => setAssetAutoActivate(e.target.checked)}
                      className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                    />
                    <label htmlFor="autoAct" className="text-[11px] text-slate-600 dark:text-slate-400 cursor-pointer">
                      Aktivasi otomatis
                    </label>
                  </div>

                  <Button
                    onClick={handleAssetUpload}
                    disabled={uploadingAsset}
                    size="sm"
                    className="h-8 text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium"
                  >
                    {uploadingAsset ? 'Mengunggah...' : 'Upload & Ekstrak'}
                  </Button>
                </div>
              </div>
            )}
          </div>

          {/* Installed Assets List */}
          <div className="p-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-3 shadow-2xs">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                Asset Terpasang di WordPress
              </h3>
              <Button
                variant="ghost"
                size="sm"
                onClick={fetchAssets}
                disabled={loadingAssets}
                className="h-6 text-[11px] px-2 text-slate-500"
              >
                <RefreshCw className={`h-3 w-3 mr-1 ${loadingAssets ? 'animate-spin' : ''}`} />
                Segarkan
              </Button>
            </div>

            {assets.length === 0 ? (
              <div className="p-6 text-center text-xs text-slate-400 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl">
                Belum ada theme atau plugin kustom yang diinstal via Webmaster.
              </div>
            ) : (
              <div className="divide-y divide-slate-100 dark:divide-slate-800">
                {assets.map((asset) => (
                  <div key={asset.id} className="py-2.5 flex items-center justify-between text-xs">
                    <div className="flex items-center gap-2.5">
                      <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400">
                        <FileArchive className="h-4 w-4" />
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-slate-800 dark:text-slate-200">
                            {asset.name || asset.slug}
                          </span>
                          <Badge className="text-[10px] uppercase font-mono">
                            {asset.kind}
                          </Badge>
                          {asset.version && (
                            <span className="text-[10px] text-slate-400">v{asset.version}</span>
                          )}
                        </div>
                        <p className="text-[11px] text-slate-400 mt-0.5 font-mono">
                          Slug: {asset.slug} • Diinstal: {new Date(asset.installed_at).toLocaleDateString()}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <Button
                        variant={asset.is_active ? 'outline' : 'ghost'}
                        size="sm"
                        onClick={() => handleToggleAsset(asset.id, asset.is_active)}
                        className={`h-7 text-xs gap-1 ${
                          asset.is_active
                            ? 'text-emerald-600 border-emerald-500/20 bg-emerald-500/10'
                            : 'text-slate-400 hover:text-slate-600'
                        }`}
                      >
                        {asset.is_active ? (
                          <>
                            <CheckCircle2 className="h-3 w-3" />
                            <span>Aktif</span>
                          </>
                        ) : (
                          <span>Nonaktif</span>
                        )}
                      </Button>

                      <button
                        onClick={() => handleDeleteAsset(asset.id)}
                        className="p-1.5 text-slate-400 hover:text-rose-500 transition-colors"
                        title="Hapus asset"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* ===================================================================== */}
      {/* TAB 2: Tracking Script Injector                                       */}
      {/* ===================================================================== */}
      {activeTab === 'scripts' && (
        <div className="space-y-4">
          {/* New Script Form */}
          <form
            onSubmit={handleInjectScript}
            className="p-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-3 shadow-2xs"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Code className="h-4 w-4 text-blue-500" />
                <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  Injeksi Script Tracking via mu-plugins Aman
                </h3>
              </div>
              <Badge className="text-[10px] bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20">
                Safe Hook
              </Badge>
            </div>

            {/* Presets Button Row */}
            <div className="flex flex-wrap items-center gap-1.5 pb-1">
              <span className="text-[11px] text-slate-400 mr-1">Preset:</span>
              <button
                type="button"
                onClick={() => applyScriptPreset('gtm')}
                className="px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-blue-50 hover:text-blue-600"
              >
                GTM
              </button>
              <button
                type="button"
                onClick={() => applyScriptPreset('ga4')}
                className="px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-blue-50 hover:text-blue-600"
              >
                Google Analytics 4
              </button>
              <button
                type="button"
                onClick={() => applyScriptPreset('meta')}
                className="px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-blue-50 hover:text-blue-600"
              >
                Meta Pixel
              </button>
              <button
                type="button"
                onClick={() => applyScriptPreset('custom')}
                className="px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-blue-50 hover:text-blue-600"
              >
                Custom Script
              </button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
              <div>
                <label className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                  Nama Script <span className="text-rose-500">*</span>
                </label>
                <Input
                  type="text"
                  placeholder="e.g. Google Tag Manager Container"
                  value={scriptName}
                  onChange={(e) => setScriptName(e.target.value)}
                  className="text-xs h-8"
                  required
                />
              </div>

              <div>
                <label className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                  Posisi Injeksi:
                </label>
                <select
                  value={scriptPosition}
                  onChange={(e) => setScriptPosition(e.target.value as any)}
                  className="w-full text-xs px-2.5 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200"
                >
                  <option value="head">Header (wp_head - disarankan untuk analytics)</option>
                  <option value="footer">Footer (wp_footer - disarankan untuk widget/chat)</option>
                </select>
              </div>
            </div>

            <div className="text-xs space-y-1">
              <label className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block">
                Kode Script / HTML Snippet <span className="text-rose-500">*</span>
              </label>
              <textarea
                rows={4}
                value={scriptContent}
                onChange={(e) => setScriptContent(e.target.value)}
                placeholder="<script> ... </script>"
                className="w-full text-xs p-2.5 font-mono bg-slate-950 text-slate-100 border border-slate-800 rounded-xl focus:outline-none focus:ring-1 focus:ring-blue-500 leading-relaxed"
                required
              />
            </div>

            <div className="flex items-center justify-between pt-1">
              <div className="flex items-center gap-1.5">
                <input
                  type="checkbox"
                  id="scriptEn"
                  checked={scriptEnabled}
                  onChange={(e) => setScriptEnabled(e.target.checked)}
                  className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                />
                <label htmlFor="scriptEn" className="text-xs text-slate-600 dark:text-slate-400 cursor-pointer">
                  Aktifkan langsung setelah injeksi
                </label>
              </div>

              <Button
                type="submit"
                disabled={injectingScript}
                size="sm"
                className="h-8 text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium"
              >
                <Plus className="h-3.5 w-3.5 mr-1" />
                {injectingScript ? 'Menyimpan...' : 'Injeksi Script'}
              </Button>
            </div>
          </form>

          {/* Active Scripts List */}
          <div className="p-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-3 shadow-2xs">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                Script Tracking Aktif ({scripts.length})
              </h3>
              <Button
                variant="ghost"
                size="sm"
                onClick={fetchScripts}
                disabled={loadingScripts}
                className="h-6 text-[11px] px-2 text-slate-500"
              >
                <RefreshCw className={`h-3 w-3 mr-1 ${loadingScripts ? 'animate-spin' : ''}`} />
                Segarkan
              </Button>
            </div>

            {scripts.length === 0 ? (
              <div className="p-6 text-center text-xs text-slate-400 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl">
                Belum ada script tracking yang diinjeksi.
              </div>
            ) : (
              <div className="divide-y divide-slate-100 dark:divide-slate-800">
                {scripts.map((script) => (
                  <div key={script.id} className="py-3 text-xs space-y-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-slate-800 dark:text-slate-200">
                          {script.name}
                        </span>
                        <Badge className="text-[10px] uppercase font-mono">
                          {script.position === 'head' ? 'wp_head' : 'wp_footer'}
                        </Badge>
                        <Badge className="text-[10px] bg-slate-100 dark:bg-slate-800 text-slate-500">
                          {script.type}
                        </Badge>
                      </div>

                      <div className="flex items-center gap-2">
                        <Button
                          variant={script.enabled ? 'outline' : 'ghost'}
                          size="sm"
                          onClick={() => handleToggleScript(script.id, script.enabled)}
                          className={`h-7 text-xs ${
                            script.enabled
                              ? 'text-emerald-600 border-emerald-500/20 bg-emerald-500/10'
                              : 'text-slate-400'
                          }`}
                        >
                          {script.enabled ? 'Aktif' : 'Nonaktif'}
                        </Button>

                        <button
                          onClick={() => setExpandedScriptId(expandedScriptId === script.id ? null : script.id)}
                          className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                          title="Lihat kode"
                        >
                          <Code className="h-3.5 w-3.5" />
                        </button>

                        <button
                          onClick={() => handleDeleteScript(script.id)}
                          className="p-1.5 text-slate-400 hover:text-rose-500 transition-colors"
                          title="Hapus script"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>

                    {expandedScriptId === script.id && (
                      <div className="p-3 rounded-xl bg-slate-950 text-slate-200 font-mono text-[11px] overflow-x-auto whitespace-pre border border-slate-800">
                        {script.content}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* ===================================================================== */}
      {/* TAB 3: Configuration Tweaker (wp-config.php)                          */}
      {/* ===================================================================== */}
      {activeTab === 'config' && (
        <div className="p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-4 shadow-2xs">
          <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
            <div>
              <div className="flex items-center gap-2">
                <Sliders className="h-4 w-4 text-blue-500" />
                <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  wp-config.php Constant Switcher
                </h3>
              </div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Modifikasi konstanta inti WordPress dengan verifikasi sintaks PHP otomatis sebelum disimpan.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={fetchConfig}
                disabled={loadingConfig}
                className="h-8 text-xs text-slate-600 dark:text-slate-300"
              >
                <RefreshCw className={`h-3.5 w-3.5 mr-1 ${loadingConfig ? 'animate-spin' : ''}`} />
                Segarkan
              </Button>
              <Button
                onClick={handleSaveConfig}
                disabled={savingConfig}
                size="sm"
                className="h-8 text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium"
              >
                {savingConfig ? 'Menyimpan...' : 'Simpan Perubahan'}
              </Button>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
            {/* WP_DEBUG */}
            <div className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60">
              <div>
                <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">WP_DEBUG</span>
                <p className="text-[11px] text-slate-400 mt-0.5">Aktifkan pelaporan error internal PHP WordPress.</p>
              </div>
              <input
                type="checkbox"
                checked={wpDebug}
                onChange={(e) => setWpDebug(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
              />
            </div>

            {/* WP_DEBUG_LOG */}
            <div className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60">
              <div>
                <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">WP_DEBUG_LOG</span>
                <p className="text-[11px] text-slate-400 mt-0.5">Catat error ke berkas /wp-content/debug.log.</p>
              </div>
              <input
                type="checkbox"
                checked={wpDebugLog}
                onChange={(e) => setWpDebugLog(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
              />
            </div>

            {/* WP_DEBUG_DISPLAY */}
            <div className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60">
              <div>
                <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">WP_DEBUG_DISPLAY</span>
                <p className="text-[11px] text-slate-400 mt-0.5">Tampilkan notice & warning langsung di halaman HTML.</p>
              </div>
              <input
                type="checkbox"
                checked={wpDebugDisplay}
                onChange={(e) => setWpDebugDisplay(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
              />
            </div>

            {/* SCRIPT_DEBUG */}
            <div className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60">
              <div>
                <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">SCRIPT_DEBUG</span>
                <p className="text-[11px] text-slate-400 mt-0.5">Gunakan versi unminified CSS & JS WordPress.</p>
              </div>
              <input
                type="checkbox"
                checked={scriptDebug}
                onChange={(e) => setScriptDebug(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
              />
            </div>

            {/* WP_MEMORY_LIMIT */}
            <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60 space-y-1.5">
              <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">WP_MEMORY_LIMIT</span>
              <select
                value={wpMemoryLimit}
                onChange={(e) => setWpMemoryLimit(e.target.value)}
                className="w-full text-xs px-2.5 py-1.5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200 font-mono"
              >
                <option value="128M">128M (Standard)</option>
                <option value="256M">256M (Recommended)</option>
                <option value="512M">512M (High Traffic / Elementor)</option>
                <option value="1024M">1024M (Enterprise)</option>
              </select>
            </div>

            {/* WP_MAX_MEMORY_LIMIT */}
            <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60 space-y-1.5">
              <span className="font-semibold text-slate-800 dark:text-slate-200 font-mono">WP_MAX_MEMORY_LIMIT</span>
              <select
                value={wpMaxMemoryLimit}
                onChange={(e) => setWpMaxMemoryLimit(e.target.value)}
                className="w-full text-xs px-2.5 py-1.5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-800 dark:text-slate-200 font-mono"
              >
                <option value="256M">256M</option>
                <option value="512M">512M (Recommended)</option>
                <option value="1024M">1024M (Heavy Admin Task)</option>
              </select>
            </div>
          </div>
        </div>
      )}

      {/* ===================================================================== */}
      {/* TAB 4: Webmaster Audit Action History                                 */}
      {/* ===================================================================== */}
      {activeTab === 'actions' && (
        <div className="p-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-3 shadow-2xs">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <History className="h-4 w-4 text-blue-500" />
              <h3 className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                Riwayat Aksi AI Webmaster Engine
              </h3>
            </div>
            <div className="flex items-center gap-2">
              <select
                value={filterActionType}
                onChange={(e) => setFilterActionType(e.target.value)}
                className="text-xs px-2 py-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-700 dark:text-slate-300"
              >
                <option value="all">Semua Aksi</option>
                <option value="inject_script">Inject Script</option>
                <option value="install_asset">Install Asset</option>
                <option value="config_tweak">Config Tweak</option>
                <option value="toggle_script">Toggle Script</option>
              </select>
              <Button
                variant="ghost"
                size="sm"
                onClick={fetchActions}
                disabled={loadingActions}
                className="h-6 text-[11px] px-2 text-slate-500"
              >
                <RefreshCw className={`h-3 w-3 mr-1 ${loadingActions ? 'animate-spin' : ''}`} />
                Segarkan
              </Button>
            </div>
          </div>

          {filteredActions.length === 0 ? (
            <div className="p-6 text-center text-xs text-slate-400 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl">
              Belum ada riwayat audit aksi webmaster yang tercatat.
            </div>
          ) : (
            <div className="space-y-2">
              {filteredActions.map((action) => {
                const isExpanded = expandedActionId === action.id
                return (
                  <div
                    key={action.id}
                    className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700/60 text-xs space-y-2"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Badge
                          className={`text-[10px] uppercase font-mono ${
                            action.action_type.includes('script')
                              ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
                              : action.action_type.includes('asset')
                              ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
                              : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                          }`}
                        >
                          {action.action_type}
                        </Badge>
                        <span className="text-slate-500 text-[11px]">
                          {new Date(action.applied_at).toLocaleString()}
                        </span>
                      </div>

                      <button
                        onClick={() => setExpandedActionId(isExpanded ? null : action.id)}
                        className="flex items-center gap-1 text-[11px] text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        <span>{isExpanded ? 'Tutup Payload' : 'Lihat Payload JSON'}</span>
                        {isExpanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                      </button>
                    </div>

                    {isExpanded && (
                      <div className="p-2.5 rounded-lg bg-slate-950 text-slate-200 font-mono text-[10px] overflow-x-auto whitespace-pre border border-slate-800">
                        {JSON.stringify(action.payload, null, 2)}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export const WordPressWebmasterConsole = WordPressWebmasterPanel
