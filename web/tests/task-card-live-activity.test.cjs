const assert = require('assert')

// Re-implement the pure logic from task-ui.ts to test in Node environment
const ANSI_REGEX = /\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\].*?(?:\x07|\x1b\\)|[PX^_].*?\x1b\\)/g
const SPINNER_BULLET_REGEX = /^[\s\x00-\x1f\r\b\x7f\u2800-\u28ff\u2022\u25cb\u25cf\u25aa\u25ab\u25e6\*\-\+\>\$\:\;\.\|·⋮…]+/
const NOISE_PAREN_REGEX = /\(ctrl\+[a-z]\s+to\s+expand\)/gi

function cleanTerminalText(text) {
  if (!text) return ''
  let cleaned = text
    .replace(ANSI_REGEX, '')
    .replace(NOISE_PAREN_REGEX, '')
    .replace(/[\r\b]/g, '')
    .replace(SPINNER_BULLET_REGEX, '')
    .replace(/\s+/g, ' ')
    .trim()
  return cleaned
}

function isTerminalNoiseLine(line) {
  if (!line || line.length < 3) return true
  const lower = line.toLowerCase()
  if (lower.startsWith('esc to cancel') || lower.startsWith('ctrl+')) return true
  if (lower.includes('gemini 3.') || lower.includes('claude 3.') || lower.includes('gpt-4')) return true
  if (/^[\s─\-_=\*~]+$/.test(line)) return true
  if (line === '>' || line === '$') return true
  return false
}

function cleanTaskDescription(desc) {
  if (!desc) return ''
  let cleaned = desc
    .replace(/^Cuan managed delegation[^\n]*\n?/i, '')
    .replace(/^Approved scope digest:[^\n]*\n?/i, '')
    .replace(/^Approval message:[^\n]*\n?/im, '')
    .replace(/^User authorization:[^\n]*\n?/im, '')
    .replace(/^## User attachments[\s\S]*$/im, '')
    .replace(/^Conversation:[^\n]*\n?/im, '')
    .replace(/^Report actual progress[\s\S]*$/im, '')
    .trim()
  return cleaned.replace(/\s+/g, ' ').trim()
}

function workStatus(task) {
  if (task.launch?.state === 'queued' || task.launch?.state === 'claimed') return 'Waiting to start'
  const attempt = task.latest_attempt?.status || task.execution_status
  if (attempt === 'running' || attempt === 'queued') return attempt === 'running' ? 'Working' : 'Waiting to start'
  if (task.status === 'done') return 'Completed'
  if (task.launch?.state === 'stopped') return 'Needs attention'
  if (task.launch?.state === 'cancelled') return 'Cancelled'
  if (task.status === 'blocked' || ['failed', 'interrupted'].includes(attempt || '')) return 'Needs attention'
  if (attempt === 'canceled' || attempt === 'cancelled') return 'Cancelled'
  if (task.status === 'review') return task.result_kind === 'answer' && ['succeeded', 'completed'].includes(attempt || '') ? 'Answered' : 'Ready for review'
  if (task.status === 'doing') return 'Not running'
  return 'Not started'
}

function taskLiveActivityText(task, fallbackText = 'Runner aktif...') {
  if (task.latest_progress) {
    const cleaned = cleanTerminalText(task.latest_progress)
    if (cleaned && !isTerminalNoiseLine(cleaned)) {
      return cleaned
    }
  }
  if (task.activities && task.activities.length > 0) {
    for (let i = task.activities.length - 1; i >= 0; i--) {
      const act = task.activities[i]
      if (act.activity_type === 'agent_report' && act.new_status === 'progress' && act.message) {
        const cleaned = cleanTerminalText(act.message.split('\n')[0])
        if (cleaned && !isTerminalNoiseLine(cleaned)) return cleaned
      }
    }
  }
  return fallbackText
}

function taskSummaryText(task) {
  const status = workStatus(task)
  if (status === 'Completed' || status === 'Answered') {
    const text = task.complete_note || task.summary || ''
    return text.replace(/\s+/g, ' ').trim()
  }
  if (status === 'Needs attention') {
    const text =
      task.launch?.safe_error ||
      task.latest_attempt?.error_message ||
      task.summary ||
      ''
    return text.replace(/\s+/g, ' ').trim()
  }
  const text = task.summary || cleanTaskDescription(task.description || task.body || '')
  return text.replace(/\s+/g, ' ').trim()
}

// ------------------- TESTS -------------------

console.log('--- Test 1: cleanTerminalText ---')
// ANSI + Braille Spinner
const t1 = cleanTerminalText('\x1B[16D\x1B[?25h\x1B[?25l\r\x1B[3A⢿  \x1B[90mRunning command..\x1B[m')
assert.strictEqual(t1, 'Running command..', 'Must clean ANSI and spinner')

// Bullet marker and (ctrl+o to expand)
const t2 = cleanTerminalText('• Scheduled 1 timer (ctrl+o to expand)')
assert.strictEqual(t2, 'Scheduled 1 timer', 'Must clean bullet marker and ctrl hint')

// Narrative status text
const t3 = cleanTerminalText('Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir.')
assert.strictEqual(t3, 'Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir.')

// Thought indicator
const t4 = cleanTerminalText('○ Thought for 10.1s (Creating a test...)')
assert.strictEqual(t4, 'Thought for 10.1s (Creating a test...)')
console.log('✓ cleanTerminalText passed all cases')

console.log('--- Test 2: cleanTaskDescription ---')
const rawDesc = `Cuan managed delegation v1.
Approved scope digest: dd7f7b86003826abe11dc6992236244fa4e152fd8150238373e22f3058b429a1
Perbarui tampilan subteks kartu task yang sedang running di antarmuka web cuan agar lebih informatif dan dinamis:
1. Ekstrak pesan status naratif / aktivitas terbaru runner.

Approval message: aa3d88bb-101d-4592-ab00-ef671f2d702e
User authorization: Approved: Tampilkan Live Status Naratif Terminal pada Kartu Task
## User attachments
{"name":"content.jpg"}`

const cleanedDesc = cleanTaskDescription(rawDesc)
assert(!cleanedDesc.includes('Cuan managed delegation'), 'Must strip delegation preamble')
assert(!cleanedDesc.includes('Approved scope digest'), 'Must strip scope digest')
assert(!cleanedDesc.includes('Approval message'), 'Must strip approval message footer')
assert(!cleanedDesc.includes('## User attachments'), 'Must strip user attachments footer')
assert(cleanedDesc.includes('Perbarui tampilan subteks kartu task yang sedang running'), 'Must preserve human instruction')
console.log('✓ cleanTaskDescription passed')

console.log('--- Test 3: taskLiveActivityText ---')
// Running task with clean latest_progress
const runningTaskWithProgress = {
  id: 'task-1',
  status: 'doing',
  latest_attempt: { status: 'running' },
  latest_progress: '\x1B[90m•\x1B[m Menjalankan unit test untuk internal/terminal...',
  description: rawDesc,
}
const act1 = taskLiveActivityText(runningTaskWithProgress)
assert.strictEqual(act1, 'Menjalankan unit test untuk internal/terminal...', 'Must return cleaned narrative activity')
assert(!act1.includes('Approved scope digest'), 'Must not return scope digest')

// Running task with empty latest_progress
const runningTaskEmptyProgress = {
  id: 'task-2',
  status: 'doing',
  latest_attempt: { status: 'running' },
  latest_progress: '',
  description: rawDesc,
}
const act2 = taskLiveActivityText(runningTaskEmptyProgress, 'Runner aktif...')
assert.strictEqual(act2, 'Runner aktif...', 'Must fall back to active runner indicator')
assert(!act2.includes('Approved scope digest'), 'Must NEVER fall back to scope digest while running')
console.log('✓ taskLiveActivityText passed')

console.log('--- Test 4: taskSummaryText ---')
// Completed task
const completedTask = {
  id: 'task-3',
  status: 'done',
  latest_attempt: { status: 'succeeded' },
  complete_note: 'Verifikasi sukses: Semua pengujian UI dan unit test lulus PASS.',
  summary: 'Tugas selesai',
  description: rawDesc,
}
const sumDone = taskSummaryText(completedTask)
assert.strictEqual(sumDone, 'Verifikasi sukses: Semua pengujian UI dan unit test lulus PASS.', 'Must return complete_note for completed task')

// Failed/Blocked task
const failedTask = {
  id: 'task-4',
  status: 'blocked',
  latest_attempt: { status: 'failed', error_message: 'Process exited with code 137 due to memory limit.' },
  description: rawDesc,
}
const sumFailed = taskSummaryText(failedTask)
assert.strictEqual(sumFailed, 'Process exited with code 137 due to memory limit.', 'Must return error_message for blocked task')

// Todo/Backlog task
const todoTask = {
  id: 'task-5',
  status: 'todo',
  description: rawDesc,
}
const sumTodo = taskSummaryText(todoTask)
assert(!sumTodo.includes('Approved scope digest'), 'Must strip scope digest in todo state')
assert(sumTodo.includes('Perbarui tampilan subteks kartu task yang sedang running'), 'Must show clean human instructions')
console.log('✓ taskSummaryText passed')

console.log('\nALL 4 TESTS PASSED SUCCESSFULLY!')
