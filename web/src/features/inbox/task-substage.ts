const roomSubstageMap = new Map<string, string>()

export function setRoomActiveSubstage(roomId: string, substage?: string) {
  if (!roomId) return
  if (!substage) {
    roomSubstageMap.delete(roomId)
  } else {
    roomSubstageMap.set(roomId, substage)
  }
}

export function getRoomActiveSubstage(roomId: string): string | undefined {
  if (!roomId) return undefined
  return roomSubstageMap.get(roomId)
}
