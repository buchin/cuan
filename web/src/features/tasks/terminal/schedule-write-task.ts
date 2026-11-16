/**
 * Task-yield scheduler for web terminal writes (ported from Fural).
 * Uses a single MessageChannel to yield between chunk parsing without
 * the nested setTimeout 4ms clamping delay or rAF starvation.
 */

let channel: MessageChannel | undefined
const callbacks: Array<() => void> = []

export function scheduleTerminalWriteTask(callback: () => void): void {
  if (typeof window === 'undefined' || typeof MessageChannel === 'undefined') {
    setTimeout(callback, 0)
    return
  }
  if (!channel) {
    channel = new MessageChannel()
    channel.port1.onmessage = () => callbacks.shift()?.()
  }
  callbacks.push(callback)
  channel.port2.postMessage(null)
}
