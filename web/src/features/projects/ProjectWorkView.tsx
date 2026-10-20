import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Search, Settings, CheckSquare } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { TaskSessionPage } from '../tasks/TaskSessionPage'
import { TaskStartingPage } from '../tasks/TaskStartingPage'

export function ProjectWorkView({
  project,
  tasks,
  taskId,
  sessionId,
  loading,
  error,
  onRetry,
}: {
  project: Project
  tasks: Task[]
  taskId?: string
  sessionId?: string
  loading: boolean
  error: string
  onRetry: () => void
}) {
  const [query, setQuery] = useState('')
  const location = useLocation()
  const base = '/projects/' + encodeURIComponent(project.id) + '/tasks'
  const scoped = tasks.filter(
    (t) =>
      t.project_id === project.id ||
      t.project_ids?.includes(project.id) ||
      (project.id === 'default' && !t.project_id && !t.project_ids?.length),
  )
  const visible = scoped.filter((t) =>
    [t.title, t.status].join(' ').toLowerCase().includes(query.toLowerCase()),
  )
  const selected = scoped.find((t) => t.id === taskId)
  return (
    <section
      aria-label="Project work"
      className="flex h-full min-h-0 min-w-0 overflow-hidden"
    >
      <aside
        aria-label="Project task list"
        className={`min-h-0 w-full min-w-0 flex-col min-[1200px]:w-80 min-[1200px]:shrink-0 min-[1200px]:border-r ${taskId ? 'hidden min-[1200px]:flex' : 'flex'}`}
      >
        <div className="shrink-0 space-y-3 border-b p-3">
          <div className="flex items-center justify-between gap-2">
            <p className="text-sm font-semibold">Work · {scoped.length}</p>
            <Link
              className="icon-action"
              aria-label="Project settings"
              to={'/projects/' + encodeURIComponent(project.id) + '/settings'}
            >
              <Settings className="h-4 w-4" />
            </Link>
          </div>
          <label className="flex min-h-11 items-center gap-2 rounded-lg border px-3">
            <Search className="h-4 w-4" />
            <input
              className="min-w-0 flex-1 bg-transparent text-base outline-none"
              aria-label="Search project work"
              placeholder="Search work…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {loading && !scoped.length ? (
            <p role="status" className="p-3 text-sm">
              Loading work…
            </p>
          ) : error ? (
            <div role="alert" className="p-3 text-sm">
              <p>{error}</p>
              <button className="action-secondary mt-3" onClick={onRetry}>
                Try again
              </button>
            </div>
          ) : visible.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">
              {query
                ? 'No matching work.'
                : 'No work yet. Ask an agent in Inbox or create a task.'}
            </p>
          ) : (
            visible.map((t) => (
              <Link
                key={t.id}
                to={base + '/' + encodeURIComponent(t.id)}
                state={{ ...location.state, taskReturnTo: base }}
                aria-current={t.id === taskId ? 'page' : undefined}
                className={`block min-h-20 border-b p-3 hover:bg-accent ${t.id === taskId ? 'bg-accent' : ''}`}
              >
                <p className="truncate text-sm font-semibold">{t.title}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t.status === 'doing'
                    ? 'In progress'
                    : t.status === 'done'
                      ? 'Completed'
                      : t.status === 'blocked'
                        ? 'Needs attention'
                        : t.status === 'review'
                          ? 'Result ready'
                          : 'Ready'}
                </p>
              </Link>
            ))
          )}
        </div>
      </aside>
      <div
        className={`min-h-0 min-w-0 flex-1 overflow-hidden ${taskId ? '' : 'hidden min-[1200px]:block'}`}
      >
        {taskId ? (
          selected ? (
            sessionId === 'start' ? (
              <TaskStartingPage key={taskId} taskId={taskId} />
            ) : (
              <TaskSessionPage
                key={taskId}
                taskId={taskId}
                sessionId={sessionId || 'latest'}
              />
            )
          ) : loading ? (
            <p className="p-6" role="status">
              Loading work…
            </p>
          ) : (
            <div className="p-6">
              <p role="alert">Work not found in this project.</p>
              <Link className="action-secondary mt-3" to={base}>
                Back to project work
              </Link>
            </div>
          )
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-sm text-muted-foreground">
            <CheckSquare className="h-8 w-8" />
            <p>Select work to view its progress and result.</p>
            <Link className="action-secondary" to="/inbox">
              Open Inbox
            </Link>
          </div>
        )}
      </div>
    </section>
  )
}
