import { Link, useLocation } from 'react-router-dom'
import type { TaskDetail } from '../../api/types'
export function GitHubPrerequisite({ task }: { task: TaskDetail }) {
 const location = useLocation()
 const state = [...(task.activities || [])].filter(item => ['github_connection_required', 'github_connection_resolved'].includes(item.activity_type)).sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0]
 if (state?.activity_type !== 'github_connection_required') return null
 const target = /^\/tasks\/[A-Za-z0-9_-]+(?:\/sessions\/[A-Za-z0-9_-]+)?$/.test(location.pathname) ? location.pathname : '/tasks/' + encodeURIComponent(task.id)
 return <aside className="mx-3 my-2 flex shrink-0 flex-wrap items-center gap-3 rounded-lg border bg-card p-3" aria-label="GitHub connection required">
  <p className="min-w-0 flex-1 text-sm">Connect GitHub to save this task branch. Your local work is retained.</p>
  <Link className="action-primary" to={'/settings/integrations/github?return_to=' + encodeURIComponent(target)}>Connect GitHub</Link>
 </aside>
}
