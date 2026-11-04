import type { ChatContact } from '../../api/chat'
type Counts = Pick<ChatContact, 'task_count' | 'done_tasks' | 'active_tasks' | 'queued_tasks' | 'review_tasks' | 'failed_tasks' | 'canceled_tasks'>

/** Only compact recognizable system reports; ordinary user/agent text stays intact. */
export function workConversationPreview(message: string, counts?: Counts): string | undefined {
  const text=message.replace(/^\s*\[@[^\]\r\n]{1,100}\]\s*/, '').replace(/[*#]/g,'').trim()
  const reportHeader=/^(?:Evaluasi\s+(?:Kriteria\s+Selesai|Definition\s+of\s+Done|DoD)|Definition\s+of\s+Done(?:\s*\(DoD\))?\s+Evaluation|DoD\s+Evaluation|Acceptance\s+Criteria\s+Evaluation|This review (?:was reported as passed|recorded items needing attention))/i.test(text)
  const report = reportHeader && (/\bStatus\s*:/i.test(text) || /^This review /i.test(text))
  const canceled=/^(?:(?:Pekerjaan|Tugas)\s+(?:telah\s+)?dibatalkan|(?:The\s+)?work\s+(?:(?:has\s+been|was)\s+)?cancel(?:led|ed)|Task\b[^\n]{0,180}\bhas been cancel(?:led|ed))/i.test(text)
  const stopped=/^(?:The work has stopped\.|Pekerjaan (?:telah |sudah )?dihentikan\.)/i.test(text)
  if(!report&&!canceled&&!stopped)return undefined
  const status=text.match(/\bStatus\s*:\s*([^\n]+)/i)?.[1]||text
  const negative=/\b(?:belum\s+lulus|perlu\s+(?:perhatian|perbaikan|revisi)|terkendala|blocked|failed|gagal|revisi|revision|needs attention)\b/i.test(status)
  if(counts&&typeof counts.task_count==='number'&&counts.task_count>0){
    const total=counts.task_count-(counts.canceled_tasks||0)
    if((counts.active_tasks||0)>0)return 'Work is in progress.'
    if((counts.queued_tasks||0)>0)return 'Work is waiting to start.'
    if((counts.failed_tasks||0)>0)return 'The result needs attention.'
    if((counts.review_tasks||0)>0)return canceled||stopped?(counts.canceled_tasks||0)>0?'A task was canceled; other results are in review.':'Result ready for review.':negative?'The result needs attention.':'Result ready for review.'
    if(total>0&&counts.done_tasks===total)return (counts.canceled_tasks||0)>0?'Remaining work completed; some tasks were canceled.':'Work completed and verified.'
    if(total===0&&(counts.canceled_tasks||0)>0)return 'Work canceled. Saved work is still available.'
  }
  if(canceled||stopped)return 'Work canceled. Saved work is still available.'
  // The message reports a review outcome; it is not independent proof of completion.
  if(negative)return 'The result needs attention.'
  if(/\b(?:lulus|passed|pass|success)\b/i.test(status))return 'Review passed. Open the conversation for details.'
  return 'Result ready for review.'
}
