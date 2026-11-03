// Settings navigation follows the route hierarchy, even on a direct deep link.
// Carry the original work context through parents; only the index returns to it.
export function getSettingsBackTarget(pathname: string, search = ''): { to: string; label: string } | undefined {
  const parts = pathname.split('/').filter(Boolean)
  if (!['settings', 'account'].includes(parts[0])) return undefined
  if (parts.length > 1) {
    const parent = '/' + parts.slice(0, -1).join('/')
    return { to: parent + search, label: parts.length > 2 ? (parts[parts.length - 2] === 'integrations' ? 'Back to integrations' : 'Back') : parts[0] === 'account' ? 'Back to Account' : 'Back to Settings' }
  }
  const query = new URLSearchParams(search)
  const returnTo = query.get('returnTo') || query.get('return_to') || ''
  if (returnTo === '/chat' || returnTo === '/studio' || /^\/inbox(?:\/[A-Za-z0-9_-]+)?(?:\?[^#]*)?$/.test(returnTo) || /^\/tasks\/[A-Za-z0-9_-]+(?:\/sessions\/[A-Za-z0-9_-]+)?$/.test(returnTo)) {
    return { to: returnTo, label: returnTo.startsWith('/tasks/') ? 'Back to task' : 'Back to conversation' }
  }
  return undefined
}
