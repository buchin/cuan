import type { Attachment, TaskActivity, TaskDetail } from '../../api/types'
import { taskAgentPending } from './task-ui'

export interface WorkEvent {
  id: string
  title: string
  body?: string
  report?: string
  at?: string
  files?: Attachment[]
  tone?: 'neutral' | 'warning' | 'success'
  pulse?: boolean
}

/** A user timeline of saved facts; raw diagnostics remain available separately. */
export function workTimeline(task: TaskDetail, id: boolean): { events: WorkEvent[]; technical: TaskActivity[] } {
  const events: WorkEvent[] = [], technical: TaskActivity[] = []
  const text = (a: string, b: string) => id ? a : b
  const latest = task.latest_attempt
  for (const activity of task.activities) {
    const base = { id: activity.id, at: activity.created_at }
    switch (activity.activity_type) {
      case 'created':
      case 'task_created':
        events.push({ ...base, title: text('Pekerjaan dibuat', 'Work created') }); break
      case 'agent_report': {
        if (activity.message.startsWith('Result saved; output file delivery is pending.')) break
        const report = ['done', 'review', 'blocked'].includes(activity.new_status || '')
        events.push({ ...base, title: report ? text('Laporan agen disimpan', 'Agent report saved') : '', ...(report ? {report: activity.message} : {body: activity.message}) }); break
      }
      case 'coordinator_review_assessment': {
        try {
          const assessment = JSON.parse(activity.message).assessment
          if (!assessment || typeof assessment.finding !== 'string') throw Error()
          events.push({ ...base, title: assessment.decision === 'pass' ? text('Hasil diperiksa', 'Result reviewed') : assessment.manager_action === 'replan' ? text('Menyesuaikan langkah', 'Replanning') : text('Pemeriksaan hasil', 'Result review'), body: assessment.finding, report: assessment.manager_reason || undefined })
        } catch { technical.push(activity) }
        break
      }
      case 'work_files':
        try {
          const { attachments } = JSON.parse(activity.message)
          if (!Array.isArray(attachments) || !attachments.length) throw Error()
          events.push({ ...base, title: text('Berkas pekerjaan', 'Work files'), files: attachments })
        } catch { technical.push(activity) }
        break
      case 'comment':
        if (['runner','system','coordinator'].includes(activity.author) || task.attempts.some(attempt=>attempt.runner_id===activity.author) || activity.message.startsWith('Revision requested:')) technical.push(activity)
        else events.push({ ...base, title: text('Catatan', 'Note'), body: activity.message })
        break
      default: technical.push(activity)
    }
  }
  for (const attempt of task.attempts) {
    if (attempt.started_at) events.push({ id: `${attempt.id}:start`, at: attempt.started_at, title: text('Pengerjaan dimulai', 'Execution started') })
    if (attempt.finished_at) {
      const stopped = attempt.status !== 'succeeded'
      events.push({ id: `${attempt.id}:finish`, at: attempt.finished_at, title: stopped ? (attempt.status === 'canceled' ? text('Pengerjaan dibatalkan', 'Execution cancelled') : text('Pengerjaan terhenti', 'Execution stopped')) : text('Eksekusi selesai', 'Execution finished'), report: attempt.error_message || undefined, tone: stopped ? 'warning' : 'neutral' })
    }
  }
  events.sort((a,b) => Date.parse(a.at || '') - Date.parse(b.at || '') || a.id.localeCompare(b.id))
  if (task.review_state) events.push({ id: 'current', title: task.review_state === 'running' ? text('Memeriksa hasil', 'Checking result') : text('Menunggu pemeriksaan', 'Waiting for review'), pulse: taskAgentPending(task) })
  else if (latest?.status === 'running' || latest?.status === 'queued' || task.launch?.state === 'queued' || task.launch?.state === 'claimed') events.push({ id: 'current', title: latest?.status === 'running' ? text('Dikerjakan', 'Working') : text('Dalam antrean', 'Queued'), pulse: taskAgentPending(task) })
  else if (task.status === 'done') events.push({ id: 'current', title: text('Pekerjaan selesai', 'Work completed'), tone: 'success' })
  return { events, technical }
}
