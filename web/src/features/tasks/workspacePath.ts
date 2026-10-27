export function workspacePath(taskId: string, pathname: string) {
  const match = pathname.match(/^\/projects\/([^/]+)\/tasks(?:\/|$)/)
  return match
    ? `/projects/${match[1]}/tasks/${encodeURIComponent(taskId)}`
    : `/tasks/${encodeURIComponent(taskId)}`
}
