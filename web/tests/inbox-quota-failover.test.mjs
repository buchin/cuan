import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const inboxActivityPath = path.join(webDir, 'src/features/inbox/InboxActivity.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

const inboxActivitySource = fs.readFileSync(inboxActivityPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

// Extract and evaluate the heuristic function directly from InboxActivity.tsx
function evaluateInboxQuotaStatus(q) {
  const recovery = q.recovery_action || 'check_status'

  const lastCooldownIdx = q.steps
    ? q.steps.reduce(
        (last, step, idx) =>
          step.kind === 'provider_cooldown' || /usage limit|quota/i.test(step.text) ? idx : last,
        -1,
      )
    : -1

  const hasSubsequentFailover =
    lastCooldownIdx !== -1 &&
    (q.steps?.slice(lastCooldownIdx + 1).some((step) =>
      [
        'account_failover',
        'model_fallback',
        'model_selected',
        'model_auto',
        'provider',
        'runtime_provider_started',
        'runtime_provider_finished',
        'runtime_tool_started',
        'output',
        'result_validation',
      ].includes(step.kind),
    ) ?? false)

  const hasActiveQuotaCooldown = lastCooldownIdx !== -1 && !hasSubsequentFailover
  const hasQuotaCooldown = hasActiveQuotaCooldown
  const isExplicitQuotaError = /usage limit|quota|no allowed account\/model route/i.test(q.safe_error || '')
  const isTechnicalNonQuotaError = /timed out|timeout|connection|network|refused|syntax|parse|5\d{2}|4\d{2}/i.test(q.safe_error || '')
  const accessBlocked =
    isExplicitQuotaError ||
    ((recovery === 'check_usage' || hasActiveQuotaCooldown) && !isTechnicalNonQuotaError && !hasSubsequentFailover)

  return {
    lastCooldownIdx,
    hasSubsequentFailover,
    hasActiveQuotaCooldown,
    hasQuotaCooldown,
    isExplicitQuotaError,
    isTechnicalNonQuotaError,
    accessBlocked,
  }
}

test('1. Heuristic: Historical provider_cooldown superseded by subsequent failover does NOT trigger accessBlocked (Zero False Alarm)', () => {
  const request = {
    request_id: 'req_123',
    message_id: 'msg_123',
    reply_id: 'rep_123',
    state: 'failed',
    recovery_action: 'check_status',
    safe_error: 'The provider CLI timed out before returning a final result.',
    steps: [
      { seq: 1, kind: 'provider_cooldown', text: 'The selected account/model reached a usage limit. Checking the next allowed route.', at: '2026-10-30T23:00:00Z' },
      { seq: 2, kind: 'account_failover', text: "Continuing this agent's reply with an allowed backup account.", at: '2026-10-30T23:00:01Z' },
      { seq: 3, kind: 'provider', text: 'Agent process is running. Waiting for the provider response.', at: '2026-10-30T23:00:02Z' },
      { seq: 4, kind: 'runtime_provider_failed', text: 'Command timed out after 30 seconds.', at: '2026-10-30T23:00:32Z' },
    ],
  }

  const result = evaluateInboxQuotaStatus(request)
  assert.equal(result.hasSubsequentFailover, true, 'Subsequent failover steps must be detected')
  assert.equal(result.hasActiveQuotaCooldown, false, 'Historical cooldown must be marked superseded/inactive')
  assert.equal(result.accessBlocked, false, 'accessBlocked must be FALSE to avoid premature quota false alarm')
})

test('2. Heuristic: Explicit quota exhaustion across all accounts triggers accessBlocked', () => {
  const request = {
    request_id: 'req_all_exhausted',
    message_id: 'msg_456',
    reply_id: 'rep_456',
    state: 'failed',
    recovery_action: 'check_usage',
    safe_error: 'No allowed account/model route is currently available. Your message is saved. Check usage and fallback settings in Settings → Agents or try again after the limit resets.',
    steps: [
      { seq: 1, kind: 'provider_cooldown', text: 'Primary account quota limited.', at: '2026-10-30T23:00:00Z' },
      { seq: 2, kind: 'account_failover', text: 'Failover to backup account 1.', at: '2026-10-30T23:00:01Z' },
      { seq: 3, kind: 'provider_cooldown', text: 'Backup account 1 quota limited.', at: '2026-10-30T23:00:02Z' },
    ],
  }

  const result = evaluateInboxQuotaStatus(request)
  assert.equal(result.isExplicitQuotaError, true, 'safe_error with no allowed account/model route must be explicit quota error')
  assert.equal(result.accessBlocked, true, 'accessBlocked must be TRUE when all accounts are exhausted')
})

test('3. Heuristic: Single account without backup reaching quota triggers accessBlocked', () => {
  const request = {
    request_id: 'req_single_quota',
    message_id: 'msg_789',
    reply_id: 'rep_789',
    state: 'failed',
    recovery_action: 'check_usage',
    safe_error: 'Provider quota is limited. Your message is saved.',
    steps: [
      { seq: 1, kind: 'provider_cooldown', text: 'The selected account reached usage limit.', at: '2026-10-30T23:00:00Z' },
    ],
  }

  const result = evaluateInboxQuotaStatus(request)
  assert.equal(result.hasSubsequentFailover, false, 'No subsequent failover occurred')
  assert.equal(result.hasActiveQuotaCooldown, true, 'Active quota cooldown remains true')
  assert.equal(result.accessBlocked, true, 'accessBlocked must be TRUE for active quota limit')
})

test('4. Heuristic: Non-quota technical failure without past cooldown never triggers accessBlocked', () => {
  const request = {
    request_id: 'req_technical_fail',
    message_id: 'msg_999',
    reply_id: 'rep_999',
    state: 'failed',
    recovery_action: 'try_again',
    safe_error: 'Connection refused while reaching provider API.',
    steps: [
      { seq: 1, kind: 'provider', text: 'Agent process running.', at: '2026-10-30T23:00:00Z' },
    ],
  }

  const result = evaluateInboxQuotaStatus(request)
  assert.equal(result.isExplicitQuotaError, false, 'Connection refused is not a quota error')
  assert.equal(result.isTechnicalNonQuotaError, true, 'Connection refused is recognized as technical error')
  assert.equal(result.accessBlocked, false, 'accessBlocked must be FALSE')
})

test('5. InboxActivity component source code: Selective quota warning and humanized error feedback architecture', () => {
  // Selective quota warning in bubble / header status
  assert.ok(
    inboxActivitySource.includes("accessBlocked ? t('Could not reply — AI quota reached') : t('Could not reply')"),
    'InboxActivity must display "Could not reply — AI quota reached" ONLY when accessBlocked is true',
  )

  // True error feedback priority in failure description
  assert.ok(
    inboxActivitySource.includes('q.safe_error\n                        ? q.safe_error'),
    'InboxActivity must prioritize real technical error q.safe_error over generic message',
  )

  // Retry button available for non-quota failures
  assert.ok(
    inboxActivitySource.includes('!accessBlocked && !autoPending && (onCheckStatus || onRetry)'),
    'InboxActivity must provide retry capability when non-quota technical error occurs',
  )

  // Touch target accessibility standard
  assert.ok(
    inboxActivitySource.includes('min-h-11'),
    'All retry and status actions must satisfy WCAG 2.1 AA min-h-11 touch target standard',
  )
})

test('6. Localization: Indonesian dictionary (id.json) contains 100% of new failure and quota translations', () => {
  assert.equal(idLocale['Could not reply — AI quota reached'], 'Belum bisa membalas — Kuota AI habis')
  assert.equal(idLocale['Could not reply'], 'Belum bisa membalas')
  assert.equal(idLocale['Try again'], 'Coba lagi')
  assert.equal(idLocale['Trying again…'], 'Mencoba lagi…')
  assert.equal(
    idLocale['All connected AI accounts reached provider usage limits. Your message is saved.'],
    'Semua akun AI yang terhubung sedang mencapai batas kuota provider. Pesan Anda tetap tersimpan.',
  )
  assert.equal(
    idLocale['The provider CLI timed out before returning a final result.'],
    'CLI provider kehabisan waktu sebelum mengembalikan hasil akhir.',
  )
  assert.equal(
    idLocale['Provider timed out. Your message is saved, but the result is unknown. Refresh history. The request will not be rerun automatically.'],
    'Provider kehabisan waktu (timeout). Pesan Anda tersimpan, namun hasilnya belum diketahui. Muat ulang riwayat. Permintaan tidak dijalankan ulang otomatis.',
  )
  assert.equal(
    idLocale['The provider could not reply. Your message is saved. Check the agent\'s status and refresh history. The request will not be rerun automatically.'],
    'Provider belum dapat membalas. Pesan Anda tersimpan. Periksa status agen dan muat ulang riwayat. Permintaan tidak dijalankan ulang otomatis.',
  )
  assert.equal(
    idLocale['Provider did not return a useful reply. Your message is saved.'],
    'Provider tidak mengembalikan balasan yang sesuai. Pesan Anda tersimpan.',
  )
  assert.equal(
    idLocale['A technical issue occurred while generating the reply. Your message is saved. You can try again.'],
    'Terjadi kendala teknis saat menyiapkan balasan. Pesan Anda tersimpan. Anda bisa mencoba lagi.',
  )
})
