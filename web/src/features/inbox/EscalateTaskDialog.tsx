import { useLanguage } from '../../context/LanguageContext'
import { useState, useEffect } from 'react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { FormField, Input, Select, Textarea } from '../../components/ui/forms'
import { apiFetch } from '../../api/client'
import { projectsApi } from '../../api/projects'
import type { Project, Task } from '../../api/types'
import type { ChatMessage } from '../../api/chat'
import { EyeOff, AlertCircle } from 'lucide-react'

export function EscalateTaskDialog({
  open,
  message,
  conversationId,
  onClose,
  onTaskCreated,
}: {
  open: boolean
  message: ChatMessage | null
  conversationId: string
  onClose: () => void
  onTaskCreated: (task: Task) => void
}) {
  const { t } = useLanguage()
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [loadingProjects, setLoadingProjects] = useState(false)

  useEffect(() => {
    if (!open || !message) return
    let live = true
    const text = message.content || message.message || ''
    const firstLine = text.trim().split('\n')[0] || ''
    setTitle(firstLine.slice(0, 100))
    setDescription(text.trim())
    setError('')
    setBusy(false)

    setLoadingProjects(true)
    projectsApi
      .list()
      .then((data) => {
        if (!live) return
        const active = data.filter((p) => !p.is_archived)
        setProjects(active)
        if (active.length > 0) {
          setProjectId(active[0].id)
        }
      })
      .catch(() => {
        if (live) setError(t('Could not load projects. Please try again.'))
      })
      .finally(() => {
        if (live) setLoadingProjects(false)
      })

    return () => {
      live = false
    }
  }, [open, message, t])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!message?.uuid || !projectId || !title.trim() || busy) return
    setBusy(true)
    setError('')
    try {
      const data = await apiFetch<{ task: Task }>(
        `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(message.uuid)}/task`,
        {
          method: 'POST',
          body: JSON.stringify({
            project_id: projectId,
            title: title.trim(),
            description: description.trim(),
          }),
        }
      )
      onTaskCreated(data.task)
      onClose()
    } catch (err: any) {
      setError(err?.message || t('Could not escalate task. Please try again.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <ActionDialog
      open={open}
      title={t('Escalate to task')}
      onClose={onClose}
      busy={busy}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Invisible notification banner */}
        <div className="rounded-lg bg-muted/60 border border-border p-3 text-xs text-muted-foreground flex items-start gap-2.5">
          <EyeOff className="h-4 w-4 shrink-0 text-primary mt-0.5" />
          <div className="space-y-0.5">
            <p className="font-semibold text-foreground">
              {t('Internal · Invisible to visitor')}
            </p>
            <p>
              {t(
                'This task is delegated to the dev team or AI agent in the background. No messages or notifications are sent to the visitor.'
              )}
            </p>
          </div>
        </div>

        {error && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-xs text-destructive"
          >
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <FormField label={t('Task title')} htmlFor="escalate-title" required>
          <Input
            id="escalate-title"
            type="text"
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            disabled={busy}
            placeholder={t('Enter task title')}
          />
        </FormField>

        <FormField label={t('Project')} htmlFor="escalate-project" required>
          <Select
            id="escalate-project"
            required
            value={projectId}
            onChange={(e) => setProjectId(e.target.value)}
            disabled={busy || loadingProjects}
          >
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name || p.id}
              </option>
            ))}
          </Select>
        </FormField>

        <FormField label={t('Instructions / Description')} htmlFor="escalate-desc">
          <Textarea
            id="escalate-desc"
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={busy}
            placeholder={t('Enter details for the developer or agent')}
          />
        </FormField>

        <div className="flex items-center justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            disabled={busy}
            className="action-secondary text-xs px-3 py-2"
          >
            {t('Cancel')}
          </button>
          <button
            type="submit"
            disabled={busy || !title.trim() || !projectId}
            className="action-primary text-xs px-4 py-2 font-medium"
          >
            {busy ? t('Creating task…') : t('Escalate to task')}
          </button>
        </div>
      </form>
    </ActionDialog>
  )
}
