export function workspacePath(taskId: string, _pathname?: string) {
  return `/tasks/${encodeURIComponent(taskId)}`
}
