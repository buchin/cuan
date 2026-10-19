import { MoreHorizontal } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
import { projectsApi } from '../../api/projects'
import { Project, Task } from '../../api/types'

export function InboxTaskAction({
  conversationId,
  messageId,
  content,
  linkedTask,
  compact = false,
}: {
  conversationId: string
  messageId: string
  content: string
  linkedTask?: Task
  compact?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('')
  const [title, setTitle] = useState(content.slice(0, 120))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<Task | null>(null)
  const prepare = async () => {
    setOpen(true)
    setBusy(true)
    setError('')
    try {
      const data = await projectsApi.list()
      setProjects(data.filter((p) => !p.is_archived))
      if (data.filter((p) => !p.is_archived).length === 1)
        setProjectId(data.filter((p) => !p.is_archived)[0].id)
      try {
        setCreated(await apiFetch<Task>(`/api/v1/tasks/msg_${messageId}`))
      } catch (e: any) {
        if (e.status !== 404) throw e
      }
    } catch {
      setError('Project atau task belum dapat dimuat. Coba buka kembali.')
    } finally {
      setBusy(false)
    }
  }
  const existing = linkedTask || created
  return (
    <div
      className={
        compact && !open && !existing
          ? 'wa-task-action text-sm'
          : open
            ? 'mt-2 w-full max-w-md text-sm'
            : 'mt-1 text-sm'
      }
    >
      {!open && existing && (
        <Link
          className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline"
          to={`/tasks/${existing.id}`}
        >
          Task: {existing.title} · {existing.status}
        </Link>
      )}
      {!open && !existing && (
        <button
          aria-label="Buat / lihat task"
          title="Buat task dari pesan"
          className="message-action-trigger inline-flex min-h-11 min-w-11 items-center justify-center gap-1 rounded-md px-2 text-sm text-muted-foreground hover:bg-muted"
          onClick={prepare}
        >
          <MoreHorizontal className="h-4 w-4" />
          {!compact && <span>Task</span>}
        </button>
      )}
      {open && (
        <div className="rounded-lg border border-border bg-card p-3 space-y-2 text-card-foreground">
          {existing ? (
            <>
              <p>Task tersimpan · {existing.status}</p>
              <Link className="underline" to={`/tasks/${existing.id}`}>
                Buka task dan sesi eksekusi
              </Link>
            </>
          ) : (
            <form
              className="space-y-2"
              onSubmit={async (e) => {
                e.preventDefault()
                setBusy(true)
                setError('')
                try {
                  const data = await apiFetch<{ task: Task }>(
                    `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/${messageId}/task`,
                    {
                      method: 'POST',
                      body: JSON.stringify({ project_id: projectId, title }),
                    },
                  )
                  setCreated(data.task)
                } catch (e: any) {
                  setError(e.message || 'Task belum dapat disimpan.')
                } finally {
                  setBusy(false)
                }
              }}
            >
              <label className="block">
                Judul task
                <input
                  className="mt-1 field-control mt-1"
                  maxLength={200}
                  required
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                />
              </label>
              <label className="block">
                Project
                <select
                  className="mt-1 field-control mt-1"
                  required
                  value={projectId}
                  onChange={(e) => setProjectId(e.target.value)}
                >
                  <option value="">Pilih project</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
              {!busy && projects.length === 0 && (
                <p>
                  Tambahkan project terlebih dahulu melalui{' '}
                  <Link className="underline" to="/projects">
                    Projects
                  </Link>
                  .
                </p>
              )}
              <p className="text-muted-foreground">
                Pesan menjadi instruksi task. Eksekusi dimulai dari halaman task
                setelah Anda memilih agen.
              </p>
              <button
                disabled={busy || !projectId}
                className="rounded bg-primary px-3 py-2 text-primary-foreground disabled:opacity-50"
              >
                {busy ? 'Memuat…' : 'Simpan task'}
              </button>
            </form>
          )}
          {error && (
            <p role="alert" className="text-destructive">
              {error}
            </p>
          )}
          <button className="action-secondary" onClick={() => setOpen(false)}>
            Tutup
          </button>
        </div>
      )}
    </div>
  )
}
