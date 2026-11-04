export type TerminalGrid = { cols: number; rows: number }

// One geometry calculation, as in Fural: fit whole measured cells without
// alternately rounding up and using FitAddon.fit's scrollbar reservation.
export function terminalGrid(width: number, height: number, cellWidth: number, cellHeight: number): TerminalGrid | null {
  if (![width, height, cellWidth, cellHeight].every(value => Number.isFinite(value) && value > 0)) return null
  const cols = Math.floor(width / cellWidth), rows = Math.floor(height / cellHeight)
  return cols >= 2 && rows >= 1 ? { cols, rows } : null
}

export class TerminalResizeGate {
  private socket: object | null = null
  private previous: TerminalGrid | null = null
  send(socket: object, grid: TerminalGrid, send: (grid: TerminalGrid) => void): boolean {
    if (this.socket === socket && this.previous?.cols === grid.cols && this.previous.rows === grid.rows) return false
    send(grid)
    this.socket = socket
    this.previous = grid
    return true
  }
}
