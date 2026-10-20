import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { Camera, Images, FolderOpen } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useRef, useState } from 'react'
import { Plus, X, FileText, LoaderCircle, Download } from 'lucide-react'
import { getAuthToken } from '../../api/client'

export interface Attachment {
  id: string
  name: string
  type: string
  size: number
  url: string
}
const accept = '.png,.jpg,.jpeg,.gif,.pdf,.txt,.md,.csv'
export function useAttachments(storageKey: string) {
  const { t } = useLanguage()

  const [items, setItems] = useState<Attachment[]>([])
  const [busy, setBusy] = useState(false)
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')
  const key = useRef(storageKey),
    uploading = useRef(false)
  key.current = storageKey
  useEffect(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(storageKey) || '[]')
      setItems(Array.isArray(stored) ? stored.slice(0, 5) : [])
    } catch {
      setItems([])
    }
    setError('')
  }, [storageKey])
  function store(next: Attachment[], target = storageKey) {
    localStorage.setItem(target, JSON.stringify(next))
    if (key.current === target) setItems(next)
  }
  async function add(files: FileList | File[]) {
    if (uploading.current) return
    const target = storageKey
    let current: Attachment[]
    try {
      current = JSON.parse(localStorage.getItem(target) || '[]')
    } catch {
      current = []
    }
    if (!Array.isArray(current)) current = []
    if (current.length + files.length > 5) {
      setError(t('Attach up to 5 files per message.'))
      return
    }
    uploading.current = true
    setBusy(true)
    setError('')
    try {
      for (const file of Array.from(files)) {
        if (file.size === 0 || file.size > 10 * 1024 * 1024)
          throw new Error(t('Choose a nonempty file up to 10 MB.'))
        if (!/\.(png|jpe?g|gif|pdf|txt|md|csv)$/i.test(file.name))
          throw new Error(t('Use PNG, JPEG, GIF, PDF, TXT, Markdown or CSV.'))
        setProgress(0)
        const uploaded = await new Promise<Attachment>((resolve, reject) => {
          const xhr = new XMLHttpRequest()
          xhr.open('POST', '/api/v1/chat/attachments')
          xhr.setRequestHeader('Authorization', 'Bearer ' + getAuthToken())
          xhr.timeout = 60000
          xhr.upload.onprogress = (e) => {
            if (e.lengthComputable)
              setProgress(Math.round((e.loaded / e.total) * 100))
          }
          xhr.onload = () => {
            let data
            try {
              data = JSON.parse(xhr.responseText)
            } catch {
              reject(new Error(t('Upload failed. Try again.')))
              return
            }
            if (xhr.status >= 200 && xhr.status < 300) resolve(data)
            else reject(new Error(data.error || t('Upload failed. Try again.')))
          }
          xhr.onerror = () =>
            reject(
              new Error(
                t('Upload failed. Check your connection and try again.'),
              ),
            )
          xhr.ontimeout = () =>
            reject(new Error(t('Upload timed out. Try again.')))
          const form = new FormData()
          form.append('file', file)
          xhr.send(form)
        })
        current = [...current, uploaded]
        store(current, target)
      }
    } catch (e) {
      if (key.current === target)
        setError(
          e instanceof Error ? t(e.message) : t('Upload failed. Try again.'),
        )
    } finally {
      uploading.current = false
      setBusy(false)
    }
  }
  return {
    items,
    busy,
    progress,
    error,
    add,
    clear: () => store([]),
    acknowledge: (ids: string[], target: string) => {
      try {
        const saved: Attachment[] = JSON.parse(
          localStorage.getItem(target) || '[]',
        )
        store(
          saved.filter((a) => !ids.includes(a.id)),
          target,
        )
      } catch {
        /* History remains authoritative. */
      }
    },
    remove: (id: string) => {
      try {
        store(items.filter((x) => x.id !== id))
      } catch {
        setError(t('Browser storage is unavailable.'))
      }
    },
    onPaste: (e: React.ClipboardEvent) => {
      if (e.clipboardData.files.length) {
        e.preventDefault()
        void add(e.clipboardData.files)
      }
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      if (e.dataTransfer.files.length) void add(e.dataTransfer.files)
    },
    onDragOver: (e: React.DragEvent) => {
      if (e.dataTransfer.types.includes('Files')) e.preventDefault()
    },
  }
}
export function AttachmentPicker({
  uploads,
  disabled = false,
}: {
  uploads: ReturnType<typeof useAttachments>
  disabled?: boolean
}) {
  const { t } = useLanguage()

  const input = useRef<HTMLInputElement>(null)
  const camera = useRef<HTMLInputElement>(null),
    photos = useRef<HTMLInputElement>(null),
    files = useRef<HTMLInputElement>(null)
  const [choices, setChoices] = useState(false)
  const selected = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) void uploads.add(e.target.files)
    e.target.value = ''
    setChoices(false)
  }
  return (
    <div className="space-y-2">
      <ActionDialog
        open={choices}
        title={t('Add attachment')}
        onClose={() => setChoices(false)}
      >
        <input
          ref={camera}
          type="file"
          accept="image/jpeg,image/png"
          capture="environment"
          className="sr-only"
          tabIndex={-1}
          aria-label={t('Take a photo')}
          onChange={selected}
        />
        <input
          ref={photos}
          type="file"
          accept="image/jpeg,image/png,image/gif"
          multiple
          className="sr-only"
          tabIndex={-1}
          aria-label={t('Choose photos')}
          onChange={selected}
        />
        <input
          ref={files}
          type="file"
          accept={accept}
          multiple
          className="sr-only"
          tabIndex={-1}
          aria-label={t('Choose files')}
          onChange={selected}
        />
        <div className="space-y-1">
          {[
            { label: 'Camera', Icon: Camera, ref: camera },
            { label: 'Photos', Icon: Images, ref: photos },
            { label: 'Files', Icon: FolderOpen, ref: files },
          ].map(({ label, Icon, ref }) => (
            <button
              key={label}
              type="button"
              className="flex min-h-12 w-full items-center gap-3 rounded-lg px-3 text-sm hover:bg-accent"
              onClick={() => ref.current?.click()}
            >
              <Icon className="h-5 w-5" />
              {t(label)}
            </button>
          ))}
        </div>
      </ActionDialog>
      <div className="flex flex-wrap items-center gap-2">
        <input
          ref={input}
          type="file"
          multiple
          accept={accept}
          className="sr-only"
          tabIndex={-1}
          aria-label={t('Choose attachments')}
          disabled={disabled || uploads.busy}
          onChange={(e) => {
            if (e.target.files) void uploads.add(e.target.files)
            e.target.value = ''
          }}
        />
        <button
          type="button"
          className="icon-action"
          aria-label={t('Attach files')}
          title={t('Attach files (up to 5, 10 MB each)')}
          disabled={disabled || uploads.busy || uploads.items.length >= 5}
          onClick={() => {
            if (
              window.matchMedia('(pointer: coarse)').matches ||
              window.innerWidth < 768
            )
              setChoices(true)
            else input.current?.click()
          }}
        >
          <Plus className="h-5 w-5" />
        </button>
        {uploads.items.map((a) => (
          <div
            key={a.id}
            className="flex max-w-full items-center gap-2 rounded-lg border border-border p-2 text-xs"
          >
            <AttachmentFile attachment={a} />
            <button
              type="button"
              className="icon-action"
              aria-label={t('Remove') + ' ' + a.name}
              disabled={disabled || uploads.busy}
              onClick={() => uploads.remove(a.id)}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
        {uploads.busy && (
          <span
            role="status"
            className="flex items-center gap-2 text-xs text-muted-foreground"
          >
            <LoaderCircle className="h-4 w-4 animate-spin" />
            {t('Uploading…')}
            {uploads.progress}%
          </span>
        )}
      </div>
      {uploads.error && (
        <p role="alert" className="text-sm text-destructive">
          {uploads.error}
        </p>
      )}
    </div>
  )
}
export function AttachmentFile({ attachment: a }: { attachment: Attachment }) {
  const { t } = useLanguage()

  const [url, setURL] = useState(''),
    [failed, setFailed] = useState(false)
  const [loading, setLoading] = useState(false)
  const blobRef = useRef('')
  useEffect(() => {
    let alive = true
    setURL('')
    setFailed(false)
    if (!a.type.startsWith('image/')) return
    fetch(a.url, { headers: { Authorization: 'Bearer ' + getAuthToken() } })
      .then(async (r) => {
        if (!r.ok) throw new Error()
        return r.blob()
      })
      .then((blob) => {
        if (alive) {
          const next = URL.createObjectURL(blob)
          blobRef.current = next
          setURL(next)
        }
      })
      .catch(() => {
        if (alive) setFailed(true)
      })
    return () => {
      alive = false
      if (blobRef.current) {
        URL.revokeObjectURL(blobRef.current)
        blobRef.current = ''
      }
    }
  }, [a.id, a.url, a.type])
  async function download() {
    setLoading(true)
    setFailed(false)
    try {
      const r = await fetch(a.url, {
        headers: { Authorization: 'Bearer ' + getAuthToken() },
      })
      if (!r.ok) throw new Error()
      const u = URL.createObjectURL(await r.blob())
      const link = document.createElement('a')
      link.href = u
      link.download = a.name
      link.click()
      setTimeout(() => URL.revokeObjectURL(u), 1000)
    } catch {
      setFailed(true)
    } finally {
      setLoading(false)
    }
  }
  return (
    <div className="min-w-0 max-w-full">
      <button
        type="button"
        onClick={() => void download()}
        disabled={loading}
        className="flex min-h-11 min-w-0 max-w-full items-center gap-2 text-left underline-offset-2 hover:underline"
        aria-label={t('Download') + ' ' + a.name}
      >
        {url ? (
          <img
            src={url}
            alt={a.name}
            className="h-16 w-16 shrink-0 rounded object-cover"
          />
        ) : (
          <FileText className="h-5 w-5 shrink-0" />
        )}
        <span className="min-w-0 break-all">
          {a.name}
          <span className="block text-xs opacity-70">
            {Math.max(1, Math.ceil(a.size / 1024))} KB
          </span>
        </span>
        {loading ? (
          <LoaderCircle className="h-4 w-4 shrink-0 animate-spin" />
        ) : (
          <Download className="h-4 w-4 shrink-0" />
        )}
      </button>
      {failed && (
        <p role="alert" className="text-xs">
          {t('File unavailable. Try downloading again.')}
        </p>
      )}
    </div>
  )
}
