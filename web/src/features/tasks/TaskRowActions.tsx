import { Link } from 'react-router-dom'
import { Terminal } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { useLanguage } from '../../context/LanguageContext'

export function TaskRowActions({ task, onDelete, onChanged, children }: {
  task: Task; onDelete?: (id: string) => void | Promise<void>; onChanged?: () => void; children?: ReactNode
}) {
  const { t } = useLanguage()
  const [confirm, setConfirm] = useState<'delete' | 'cancel' | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const active = ['queued', 'running'].includes(task.latest_attempt?.status || '') || ['queued', 'doing', 'working'].includes(task.status)
  async function perform() {
    if (busy || !confirm) return
    setBusy(true); setError('')
    try {
      if (confirm === 'cancel') await tasksApi.cancel(task.id)
      else {
        if (onDelete) await onDelete(task.id)
        else await tasksApi.delete(task.id)
        window.dispatchEvent(new CustomEvent('cuan:task-deleted', { detail: { id: task.id } }))
      }
      setConfirm(null); onChanged?.()
    } catch { setError(t('Could not complete this action. Try again.')) }
    finally { setBusy(false) }
  }
  return <>
    <div className="group flex min-w-0 items-center">
    {children && <div className="min-w-0 flex-1">{children}</div>}
    {task.latest_attempt?.status === 'running' && <Link
      to={`/tasks/${encodeURIComponent(task.id)}/sessions/${encodeURIComponent(task.latest_attempt.id)}`}
      aria-label={t('Open terminal')}
      title={t('Open terminal')}
      onClick={event => event.stopPropagation()}
      className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring lg:opacity-0 lg:group-hover:opacity-100 lg:group-focus-within:opacity-100"
    ><Terminal aria-hidden="true" className="h-4 w-4" /></Link>}
    <ItemActions label={task.title} persistent actions={[
      { label: 'Copy link', run: () => copyItemLink('/tasks/' + encodeURIComponent(task.id)) },
      ...(active ? [{ label: 'Cancel task', run: () => setConfirm('cancel') }] : [{ label: 'Delete task', run: () => setConfirm('delete') }]),
    ]}>{undefined}</ItemActions>
    </div>
    <ActionDialog open={confirm !== null} title={t(confirm === 'cancel' ? 'Cancel task' : 'Delete task')} busy={busy} onClose={() => setConfirm(null)}>
      <p className="text-sm break-words">{task.title}</p>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex justify-end gap-2">
        <button type="button" className="action-secondary min-h-11" disabled={busy} onClick={() => setConfirm(null)}>{t('Keep task')}</button>
        <button type="button" className="action-secondary min-h-11 text-destructive" disabled={busy} onClick={() => void perform()}>{t(confirm === 'cancel' ? 'Cancel task' : 'Delete task')}</button>
      </div>
    </ActionDialog>
  </>
}
