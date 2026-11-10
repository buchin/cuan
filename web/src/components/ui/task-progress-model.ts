export interface TaskProgressCounts {
  /** Number of tasks still in scope, excluding cancellations. */
  total?: number
  done?: number
  canceled?: number
  active?: number
  queued?: number
  review?: number
  blocked?: number
  minTasks?: number
}

const count = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0

export function taskProgressModel({ total, done = 0, canceled = 0, active, queued, review, blocked, minTasks = 1 }: TaskProgressCounts) {
  if (!count(total) || !count(done) || !count(canceled) || total === 0 || done > total || total + canceled < minTasks) return null
  const pending = [active,queued,review].reduce<number>((sum,value)=>sum+(count(value)?value:0),0)
  if (done === 0 && pending === 0 && (!count(blocked) || blocked === 0)) return null
  const rows = [{ label: 'Completed', count: done }]
  const breakdown = [active, queued, review, blocked]
  if (breakdown.every(count) && done + active! + queued! + review! + blocked! <= total) {
    rows.push({label:'Working',count:active!},{label:'Queued',count:queued!},{label:'In review',count:review!},{label:'Needs attention',count:blocked!},
      {label:'Not started',count:total-done-active!-queued!-review!-blocked!})
  } else rows.push({label:'Remaining',count:total-done})
  if (canceled > 0) rows.push({label:'Canceled',count:canceled})
  return { total, done, canceled, pending, active: count(active) ? active : 0, fraction:done/total, complete:done===total, mixed:done===total&&canceled>0, rows:rows.filter(row=>row.count>0) }
}
