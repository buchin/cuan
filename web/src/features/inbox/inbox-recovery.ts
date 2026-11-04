// Retrying an uncertain reply starts server-side reconciliation, never a blind
// replay. Active and canceled turns cannot be restarted by this control.
export function inboxRecoveryControls(state: string, action: string | undefined, quota: boolean) {
  const recovery = action || 'check_status'
  const canRetry = (state === 'failed' || state === 'interrupted') && recovery !== 'recovering'
  const waitingForCapacity = state === 'failed' && quota && ['check_usage', 'recovering'].includes(recovery)
  return { canRetry, waitingForCapacity }
}
