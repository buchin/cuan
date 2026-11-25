import type { Attachment, TaskActivity, TaskAttempt, TaskDetail } from '../../api/types'
import { sanitizeCompletedTaskNote, taskAgentPending, workDisplayStatus } from './task-ui'

export interface WorkEvent {
  id: string
  title: string
  body?: string
  report?: string
  at?: string
  files?: Attachment[]
  tone?: 'neutral' | 'warning' | 'success' | 'review'
  pulse?: boolean
}

type AttemptStage = 'execution' | 'verification' | 'native_release'

function attemptStage(attempt: TaskAttempt, activities: TaskActivity[]): AttemptStage {
  if (attempt.kind === 'native_release') return 'native_release'
  if (attempt.kind === 'verification' || attempt.kind === 'inspection') return 'verification'
  if (attempt.kind === 'execution') return 'execution'
  const bound = activities.filter(a => a.attempt_id === attempt.id)
  if (bound.some(a => ['native_release_preparation', 'native_release_delivery', 'deployment_verified'].includes(a.activity_type))) {
    return 'native_release'
  }
  if (bound.some(a => ['live_result_verified', 'repository_scope_verified'].includes(a.activity_type))) {
    return 'verification'
  }
  return 'execution'
}

function shortCommit(sha?: string): string {
  return sha && sha.length >= 7 ? sha.slice(0, 8) : ''
}

function releaseSummaryFromActivities(task: TaskDetail, attemptId?: string, id?: boolean): string | undefined {
  const deployAct = task.activities.find(
    a => a.activity_type === 'deployment_verified' && (!attemptId || a.attempt_id === attemptId),
  )
  let commit = shortCommit(task.commit_sha)
  if (deployAct?.message) {
    try {
      const parsed = JSON.parse(deployAct.message)
      commit = shortCommit(parsed.commit || parsed.target_commit) || commit
    } catch { /* ignore */ }
  }
  if (commit) {
    return id ? `Rilis aktif pada commit ${commit}.` : `Live release active on commit ${commit}.`
  }
  return undefined
}

/** A user timeline of saved facts; raw diagnostics remain available separately. */
export function workTimeline(task: TaskDetail, id: boolean): { events: WorkEvent[]; technical: TaskActivity[] } {
  const events: WorkEvent[] = [], technical: TaskActivity[] = []
  const text = (a: string, b: string) => id ? a : b
  const latest = task.latest_attempt
  const isDone = task.status === 'done'
  for (const activity of task.activities) {
    const base = { id: activity.id, at: activity.created_at }
    switch (activity.activity_type) {
      case 'created':
      case 'task_created':
        events.push({ ...base, title: text('Pekerjaan dibuat', 'Work created') }); break
      case 'agent_report': {
        if (activity.message.startsWith('Result saved; output file delivery is pending.')) break
        const report = ['done', 'review', 'blocked'].includes(activity.new_status || '')
        const msg = sanitizeCompletedTaskNote(activity.message, isDone)
        if (!msg) break
        events.push({ ...base, title: report ? text('Laporan agen disimpan', 'Agent report saved') : '', ...(report ? { report: msg } : { body: msg }) }); break
      }
      case 'coordinator_review_assessment': {
        try {
          const assessment = JSON.parse(activity.message).assessment
          if (!assessment || typeof assessment.finding !== 'string') throw Error()
          const finding = sanitizeCompletedTaskNote(assessment.finding, isDone)
          events.push({
            ...base,
            title: assessment.decision === 'pass' ? text('Hasil diperiksa', 'Result reviewed') : assessment.manager_action === 'replan' ? text('Menyesuaikan langkah', 'Replanning') : text('Pemeriksaan hasil', 'Result review'),
            body: finding,
            report: assessment.manager_reason || undefined,
          })
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
  let hasReleaseEvent = false
  for (const attempt of task.attempts) {
    const stage = attemptStage(attempt, task.activities)
    if (stage === 'native_release') hasReleaseEvent = true
    const startTitle = stage === 'verification'
      ? text('Verifikasi dimulai', 'Verification started')
      : stage === 'native_release'
        ? text('Push & rilis live dimulai', 'Push & live release started')
        : text('Pengerjaan dimulai', 'Execution started')
    if (attempt.started_at) events.push({ id: `${attempt.id}:start`, at: attempt.started_at, title: startTitle })
    if (attempt.finished_at) {
      const stopped = attempt.status !== 'succeeded'
      const finishTitle = stage === 'verification'
        ? (stopped ? (attempt.status === 'canceled' ? text('Verifikasi dibatalkan', 'Verification cancelled') : text('Verifikasi terhenti', 'Verification stopped')) : text('Verifikasi selesai', 'Verification finished'))
        : stage === 'native_release'
          ? (stopped ? (attempt.status === 'canceled' ? text('Push & rilis live dibatalkan', 'Push & live release cancelled') : text('Push & rilis live terhenti', 'Push & live release stopped')) : text('Push & rilis live selesai', 'Push & live release finished'))
          : (stopped ? (attempt.status === 'canceled' ? text('Pengerjaan dibatalkan', 'Execution cancelled') : text('Pengerjaan terhenti', 'Execution stopped')) : text('Pengerjaan selesai', 'Execution finished'))
      const body = !stopped && stage === 'native_release' ? releaseSummaryFromActivities(task, attempt.id, id) : undefined
      events.push({ id: `${attempt.id}:finish`, at: attempt.finished_at, title: finishTitle, body, report: attempt.error_message || undefined, tone: stopped ? 'warning' : 'neutral' })
    }
  }
  if (!hasReleaseEvent) {
    const deployAct = task.activities.find(a => a.activity_type === 'deployment_verified')
    const hasLiveMarker = Boolean(deployAct) || (isDone && Boolean(task.complete_note?.includes('telah diverifikasi aktif pada rilis live') || task.complete_note?.includes('Perubahan telah berhasil diterapkan')))
    if (hasLiveMarker) {
      const at = deployAct?.created_at || task.completed_at || task.updated_at
      events.push({
        id: deployAct ? `release:${deployAct.id}` : 'release:verified',
        at,
        title: text('Push & rilis live selesai', 'Push & live release finished'),
        body: releaseSummaryFromActivities(task, deployAct?.attempt_id, id),
        tone: 'neutral',
      })
    }
  }
  events.sort((a,b) => Date.parse(a.at || '') - Date.parse(b.at || '') || a.id.localeCompare(b.id))
  const phase=workDisplayStatus(task)
  if (phase==='Waiting to deploy' || phase==='Deploying') events.push({id:'current',title:phase==='Waiting to deploy'?text('Menunggu rilis','Waiting to deploy'):text('Merilis','Deploying'),pulse:taskAgentPending(task)})
  else if (task.review_state || task.status === 'review') events.push({ id: 'current', title: task.review_state === 'running' ? text('Memeriksa hasil', 'Checking result') : text('Menunggu pemeriksaan', 'Waiting for review'), pulse: taskAgentPending(task), tone: 'review' })
  else if (latest?.status === 'running' || latest?.status === 'queued' || task.launch?.state === 'queued' || task.launch?.state === 'claimed') events.push({ id: 'current', title: latest?.status === 'running' ? text('Dikerjakan', 'Working') : text('Dalam antrean', 'Queued'), pulse: taskAgentPending(task) })
  else if (task.status === 'done') events.push({ id: 'current', title: text('Pekerjaan selesai', 'Work completed'), tone: 'success' })
  return { events, technical }
}
