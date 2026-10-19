import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
import { InboxTaskDraft } from '../../api/chat'
import { projectsApi } from '../../api/projects'
import { Project, Task } from '../../api/types'

export function InboxTaskAction({
  conversationId,
  messageId,
  draft,
  linkedTask,
}: {
  conversationId: string
  messageId: string
  draft?: InboxTaskDraft
  linkedTask?: Task
}) {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState(draft?.project_id || '')
  const [title, setTitle] = useState(draft?.title || '')
  const [description, setDescription] = useState(draft?.description || '')
  const [loading, setLoading] = useState(!!draft)
  const [busy, setBusy] = useState(false)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<Task | null>(null)
  const [retry, setRetry] = useState(0)
  const existing = linkedTask || created
  useEffect(() => {
    if (!draft || existing) return
    let live = true
    setLoading(true)
    setReady(false)
    setError('')
    Promise.all([
      projectsApi.list(),
      apiFetch<Task>(`/api/v1/tasks/msg_${messageId}`).catch((e) => {
        if (e.status === 404) return null
        throw e
      }),
    ])
      .then(([data, task]) => {
        if (!live) return
        const active = data.filter((p) => !p.is_archived)
        setProjects(active)
        setProjectId((current) =>
          active.some((p) => p.id === current)
            ? current
            : active.length === 1
              ? active[0].id
              : '',
        )
        setCreated(task)
        setReady(true)
      })
      .catch(() => {
        if (live) setError('Project atau task belum dapat dimuat. Coba lagi.')
      })
      .finally(() => {
        if (live) setLoading(false)
      })
    return () => {
      live = false
    }
  }, [draft, existing, messageId, retry])

  if (!draft && !existing) return null
  if (existing)
    return (
      <div
        className="mt-2 rounded-lg border border-border bg-card p-3 text-sm text-card-foreground"
        aria-label="Task dari percakapan"
      >
        <p className="font-medium">Task tersimpan · {existing.status}</p>
        <Link
          className="inline-flex min-h-11 items-center underline"
          to={`/tasks/${existing.id}`}
        >
          Buka task: {existing.title}
        </Link>
        <p className="text-muted-foreground">
          Eksekusi dikelola dari halaman task.
        </p>
      </div>
    )
  return (
    <form
      aria-label="Draft task"
      className="mt-3 w-full min-w-0 max-w-md space-y-3 rounded-lg border border-border bg-card p-3 text-sm text-card-foreground"
      onSubmit={async (e) => {
        e.preventDefault()
        if (busy || loading || !ready) return
        setBusy(true)
        setError('')
        try {
          const data = await apiFetch<{ task: Task }>(
            `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/${messageId}/task`,
            {
              method: 'POST',
              body: JSON.stringify({
                project_id: projectId,
                title: title.trim(),
                description: description.trim(),
              }),
            },
          )
          setCreated(data.task)
        } catch (e: any) {
          setError(
            e.message ||
              'Task belum dapat disimpan. Coba lagi; task tidak akan dibuat ganda.',
          )
        } finally {
          setBusy(false)
        }
      }}
    >
      <div>
        <p className="font-medium">Draft task</p>
        <p className="mt-1 text-muted-foreground">
          Periksa dan edit sebelum membuat task. Belum tersimpan atau
          dijalankan.
        </p>
      </div>
      <label className="block">
        Judul task
        <input
          className="field-control mt-1"
          maxLength={200}
          required
          disabled={busy}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </label>
      <label className="block">
        Deskripsi
        <textarea
          aria-label="Deskripsi"
          className="field-control mt-1 min-h-36 resize-y"
          maxLength={6000}
          required
          disabled={busy}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </label>
      <label className="block">
        Project
        <select
          aria-label="Project"
          className="field-control mt-1"
          required
          disabled={loading || busy || !ready}
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
      {loading && <p role="status">Memuat project dan status task…</p>}
      {!loading && projects.length === 0 && !error && (
        <p>
          Tambahkan project melalui{' '}
          <Link className="underline" to="/projects">
            Projects
          </Link>
          .
        </p>
      )}
      {error && (
        <div role="alert">
          <p className="text-destructive">{error}</p>
          <button
            type="button"
            className="action-secondary mt-1"
            disabled={busy || loading}
            onClick={() => setRetry((n) => n + 1)}
          >
            Coba lagi
          </button>
        </div>
      )}
      <button
        className="action-primary min-h-11 disabled:opacity-50"
        disabled={
          busy ||
          loading ||
          !ready ||
          !projectId ||
          !title.trim() ||
          !description.trim()
        }
      >
        {busy ? 'Membuat…' : 'Buat task'}
      </button>
      <p className="text-muted-foreground">
        Task masuk backlog. Membuat task tidak memulai eksekusi agent.
      </p>
    </form>
  )
}
