// Targeted unit tests for StatusReportCard and task report message parsing.
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const assert = require('node:assert/strict')
let ts
try {
  ts = require('typescript')
} catch {
  const candidates = [
    '/opt/cuan/data/projects/task-workspaces/c80ba144a5fce248fa5cf8a4f01d3d720d66ad7c2686269f0f74769c612cdb6d/web/node_modules/typescript',
    '/home/dev/.local/lib/node_modules/typescript',
  ]
  for (const c of candidates) {
    try { ts = require(c); break } catch {}
  }
}
if (!ts) throw new Error('TypeScript module not found for test transpile')

const tsFile = path.join(__dirname, 'StatusReportCard.tsx')
const rawSource = fs.readFileSync(tsFile, 'utf8')
const transpile = ts.transpileModule(rawSource, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2020,
    jsx: ts.JsxEmit.React,
  },
})

const context = {
  exports: {},
  require: (mod) => {
    if (mod === 'react') return { forwardRef: (fn) => fn }
    if (mod === 'lucide-react') return {
      CheckCircle2: 'CheckCircle2',
      AlertCircle: 'AlertCircle',
      Clock3: 'Clock3',
      Loader2: 'Loader2',
      Info: 'Info',
    }
    if (mod.endsWith('/utils')) return { cn: (...classes) => classes.filter(Boolean).join(' ') }
    throw new Error(`Unexpected require: ${mod}`)
  },
}

vm.runInNewContext(transpile.outputText, context)
const { parseTaskReportMessage, getReportStatusTheme } = context.exports

// Test 1: Verification report with all criteria met (Success)
const successMsg = `Based on the work session's verification report:

Semua kriteria penerimaan telah terpenuhi: indikator visual badge 'Working' dengan animasi pulse telah diimplementasikan dan diverifikasi pada item percakapan Inbox saat task aktif, badge diperbarui secara otomatis ketika task berhenti, dan distribusi kuota/fallback rute chat Studio telah diterapkan serta divalidasi dengan pengujian unit dan type-check tanpa error.`

const parsedSuccess = parseTaskReportMessage(successMsg, 'id')
assert.ok(parsedSuccess, 'Should parse verification report')
assert.equal(parsedSuccess.status, 'success')
assert.equal(parsedSuccess.title, 'Laporan Verifikasi')
assert.equal(parsedSuccess.badgeText, 'Berhasil')
assert.ok(parsedSuccess.body.includes('Semua kriteria penerimaan telah terpenuhi'))

// English locale check
const parsedSuccessEn = parseTaskReportMessage(successMsg, 'en')
assert.equal(parsedSuccessEn.status, 'success')
assert.equal(parsedSuccessEn.title, 'Verification Report')
assert.equal(parsedSuccessEn.badgeText, 'Success')

// Test 2: Verification report with failure (Failed/Blocked)
const failedMsg = `Based on the work session's verification report:

Pengujian gagal: 2 kriteria tidak terpenuhi dan terdapat build error pada runner.`
const parsedFailed = parseTaskReportMessage(failedMsg, 'id')
assert.ok(parsedFailed)
assert.equal(parsedFailed.status, 'failed')
assert.equal(parsedFailed.badgeText, 'Gagal')

// Test 3: Work update checking (Running)
const runningMsg = `I’m checking the latest work update. Implementasi Badge Working + Pulse di Inbox dan Optimasi Routing Kuota Agent Studio.`
const parsedRunning = parseTaskReportMessage(runningMsg, 'id')
assert.ok(parsedRunning)
assert.equal(parsedRunning.status, 'running')
assert.equal(parsedRunning.title, 'Pembaruan Pekerjaan')
assert.equal(parsedRunning.badgeText, 'Sedang Berjalan')
assert.equal(parsedRunning.body, 'Implementasi Badge Working + Pulse di Inbox dan Optimasi Routing Kuota Agent Studio.')

// Test 4: Review required (Review)
const reviewMsg = `The result still needs review. Its task and reported evidence are saved.`
const parsedReview = parseTaskReportMessage(reviewMsg, 'en')
assert.ok(parsedReview)
assert.equal(parsedReview.status, 'review')
assert.equal(parsedReview.title, 'Review Required')
assert.equal(parsedReview.badgeText, 'Needs Review')

// Test 5: Validation failed (Blocked)
const blockedMsg = `The result review could not be validated. The saved task and its evidence are preserved; completion has not been confirmed.`
const parsedBlocked = parseTaskReportMessage(blockedMsg, 'en')
assert.ok(parsedBlocked)
assert.equal(parsedBlocked.status, 'blocked')
assert.equal(parsedBlocked.title, 'Validation Blocked')
assert.equal(parsedBlocked.badgeText, 'Blocked')

// Test 6: Normal chat message should return null (NO regression on regular chat)
const regularMsg = `Halo tim, bisakah menambahkan tombol export ke CSV pada halaman report?`
const parsedRegular = parseTaskReportMessage(regularMsg)
assert.equal(parsedRegular, null, 'Normal chat messages must not be parsed as task reports')

const greetingMsg = `Good morning! How is the sprint going?`
assert.equal(parseTaskReportMessage(greetingMsg), null)

// Test 7: Theme styling tokens
const successTheme = getReportStatusTheme('success')
assert.ok(successTheme.accentBorder.includes('border-l-emerald-500'))
assert.ok(successTheme.containerBg.includes('bg-emerald-500/'))
assert.ok(successTheme.iconColor.includes('text-emerald-600'))

const failedTheme = getReportStatusTheme('failed')
assert.ok(failedTheme.accentBorder.includes('border-l-rose-500'))
assert.ok(failedTheme.containerBg.includes('bg-rose-500/'))

const reviewTheme = getReportStatusTheme('review')
assert.ok(reviewTheme.accentBorder.includes('border-l-amber-500'))
assert.ok(reviewTheme.containerBg.includes('bg-amber-500/'))

const runningTheme = getReportStatusTheme('running')
assert.ok(runningTheme.accentBorder.includes('border-l-sky-500'))
assert.ok(runningTheme.containerBg.includes('bg-sky-500/'))

console.log('PASS: StatusReportCard parser, themes, multi-locale, and regression checks verified.')
