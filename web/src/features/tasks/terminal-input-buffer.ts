// Only unsent input is buffered. Successfully handed-off bytes are never replayed.
export class TerminalInputBuffer {
  private generation = ''
  private pending = ''
  bind(generation: string) {
    if (generation !== this.generation) { this.pending = ''; this.generation = generation }
  }
  write(generation: string, data: string, send?: (data: string) => void): boolean {
    if (generation !== this.generation || this.pending.length + data.length > 65536) return false
    this.pending += data
    return send ? this.flush(generation, send) : true
  }
  flush(generation: string, send: (data: string) => void): boolean {
    if (generation !== this.generation) return false
    if (!this.pending) return true
    try { send(this.pending); this.pending = ''; return true } catch { return false }
  }
  clear() { this.pending = '' }
}
