/**
 * Connection-scoped UTF-8 decoder for raw PTY WebSocket messages (ported from Fural).
 * Preserves incomplete multi-byte sequences across WebSocket message boundaries.
 */

export class TerminalStreamDecoder {
  private decoder = new TextDecoder()

  decode(data: string | ArrayBuffer | Uint8Array): string {
    if (typeof data === 'string') {
      const pending = this.decoder.decode()
      this.decoder = new TextDecoder()
      return pending + data
    }
    return this.decoder.decode(data, { stream: true })
  }

  reset(): void {
    this.decoder = new TextDecoder()
  }
}
