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
    if (mod === 'react') return {
      forwardRef: (fn) => fn,
      useState: (initial) => [initial, () => {}],
    }
    if (mod === 'lucide-react') return {
      CheckCircle2: 'CheckCircle2',
      AlertCircle: 'AlertCircle',
      Clock3: 'Clock3',
      Loader2: 'Loader2',
      Info: 'Info',
      ShieldCheck: 'ShieldCheck',
      RotateCcw: 'RotateCcw',
      CheckSquare: 'CheckSquare',
      ListChecks: 'ListChecks',
      AlertTriangle: 'AlertTriangle',
      Terminal: 'Terminal',
      GitCommit: 'GitCommit',
      GitBranch: 'GitBranch',
      Rocket: 'Rocket',
      ArrowRight: 'ArrowRight',
      FileText: 'FileText',
      Check: 'Check',
      Copy: 'Copy',
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
const runningMsg = `Memeriksa pembaruan pekerjaan terbaru: Implementasi Badge Working + Pulse di Inbox dan Optimasi Routing Kuota Agent Studio.`
const parsedRunning = parseTaskReportMessage(runningMsg, 'id')
assert.ok(parsedRunning)
assert.equal(parsedRunning.status, 'running')
assert.equal(parsedRunning.title, 'Pembaruan Pekerjaan')
assert.equal(parsedRunning.badgeText, 'Sedang Berjalan')
assert.equal(parsedRunning.body, 'Implementasi Badge Working + Pulse di Inbox dan Optimasi Routing Kuota Agent Studio.')

// Internal review trigger cues must NOT be parsed as running work updates
assert.equal(parseTaskReportMessage('I’m checking the latest work update. Show.'), null)

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

// Test 7: DoD Format A - Coordinator Review evaluation (Pass / Lulus)
const dodFormatA = `### Evaluasi Kriteria Selesai (Definition of Done)
**Status:** LULUS

**Bukti yang Terverifikasi:**
- [x] **Dropdown category dihapus**: Komponen Select category dihapus dari CreateProjectModal.tsx
- [x] **Quick chips tersedia**: 4 chip inspirasi ditampilkan di bawah input nama project

**Belum Terbukti / Gap yang Perlu Diperbaiki:**
- [ ] **Kriteria/Fitur**: Tidak ada (seluruh kriteria terpenuhi)

**Tindakan:** Menyatakan selesai`

const parsedDoDA = parseTaskReportMessage(dodFormatA, 'id')
assert.ok(parsedDoDA, 'Should parse DoD Format A')
assert.equal(parsedDoDA.status, 'success')
assert.equal(parsedDoDA.title, 'Evaluasi Definition of Done')
assert.equal(parsedDoDA.badgeText, 'Lulus')
assert.ok(parsedDoDA.dodDetails, 'dodDetails must be populated')
assert.equal(parsedDoDA.dodDetails.hasExplicitNoGap, true)
assert.equal(parsedDoDA.dodDetails.checklist.length, 2)
assert.equal(parsedDoDA.dodDetails.checklist[0].title, 'Dropdown category dihapus')
assert.equal(parsedDoDA.dodDetails.checklist[0].completed, true)
assert.equal(parsedDoDA.dodDetails.action, 'Menyatakan selesai')

// Test 8: DoD Format B - Chat room screenshot format (Pass / Lulus with technical verification)
const dodFormatB = `Evaluasi Definition of Done (DoD)
- Task: cuan: Hapus dropdown category di buat project dan ganti dengan inspirasi chips
- Status: LULUS
- Ringkasan: Dilaporkan bahwa dropdown category telah dihapus total, digantikan oleh quick chips, dan diverifikasi dengan tes.
- Checklist DoD:
  - [x] Kriteria 1: Dropdown category dihapus
  - [x] Kriteria 2: 4 quick chips ditambahkan
- Verifikasi Teknis:
  - Commit: 63bbb6f2946c7eb075a3da4cb2ea2432a488e0b4 (cuan/task-63bbb6f2946c7eb075a3da4cb2ea2432a488e0b4)
  - Lint/Build: yarn lint / yarn build lulus tanpa error
- Gap/Tindak Lanjut:
  - [ ] Kriteria/Fitur: Tidak ada
- Tindakan: Menyatakan selesai`

const parsedDoDB = parseTaskReportMessage(dodFormatB, 'id')
assert.ok(parsedDoDB, 'Should parse DoD Format B')
assert.equal(parsedDoDB.status, 'success')
assert.equal(parsedDoDB.badgeText, 'Lulus')
assert.ok(parsedDoDB.dodDetails)
assert.equal(parsedDoDB.dodDetails.taskTitle, 'cuan: Hapus dropdown category di buat project dan ganti dengan inspirasi chips')
assert.ok(parsedDoDB.dodDetails.summary.includes('Dilaporkan bahwa dropdown category'))
assert.equal(parsedDoDB.dodDetails.checklist.length, 2)
assert.equal(parsedDoDB.dodDetails.hasExplicitNoGap, true)
assert.ok(parsedDoDB.dodDetails.technicalVerification)
assert.equal(parsedDoDB.dodDetails.technicalVerification.commit, '63bbb6f2946c7eb075a3da4cb2ea2432a488e0b4')
assert.equal(parsedDoDB.dodDetails.technicalVerification.commitShort, '63bbb6f')
assert.equal(parsedDoDB.dodDetails.technicalVerification.branch, 'cuan/task-63bbb6f2946c7eb075a3da4cb2ea2432a488e0b4')
assert.ok(parsedDoDB.dodDetails.technicalVerification.lintBuild.includes('yarn lint'))

// Test 9: DoD Format with Revision (Needs Revision) and trailing delegation text
const dodFormatRevision = `### Evaluasi Kriteria Selesai (Definition of Done)
**Status:** BELUM LULUS (Perlu Revisi)

**Bukti yang Terverifikasi:**
- [x] **Pelaksanaan tugas**: Dicatat oleh runner

**Belum Terbukti / Gap yang Perlu Diperbaiki:**
- [ ] **Deploy**: Perubahan belum ter-deploy ke server live produksi cuan.dojo.cc

**Tindakan:** Mengembalikan tugas secara otomatis ke tim eksekusi untuk diselesaikan sesuai kriteria.

Delegated revision for [cuan: Hapus dropdown](/tasks/task-123) to agent.`

const parsedDoDRev = parseTaskReportMessage(dodFormatRevision, 'id')
assert.ok(parsedDoDRev)
assert.equal(parsedDoDRev.status, 'review')
assert.equal(parsedDoDRev.badgeText, 'Perlu Revisi')
assert.equal(parsedDoDRev.dodDetails.hasExplicitNoGap, false)
assert.equal(parsedDoDRev.dodDetails.gaps.length, 1)
assert.equal(parsedDoDRev.dodDetails.gaps[0].title, 'Deploy')
assert.ok(parsedDoDRev.dodDetails.remainingText.includes('Delegated revision for'))

// Test 10: DoD Format with Blocked status
const dodFormatBlocked = `### Evaluasi Kriteria Selesai (Definition of Done)
**Status:** TERKENDALA (Blocked)

**Bukti yang Terverifikasi:**
- [x] **Kode**: Implementasi selesai

**Belum Terbukti / Gap yang Perlu Diperbaiki:**
- [ ] **Koneksi**: Database host tidak dapat diakses dari runner

**Tindakan:** Memerlukan keputusan PO`

const parsedDoDBlocked = parseTaskReportMessage(dodFormatBlocked, 'id')
assert.ok(parsedDoDBlocked)
assert.equal(parsedDoDBlocked.status, 'blocked')
assert.equal(parsedDoDBlocked.badgeText, 'Terkendala')

// Test 11: Theme styling tokens
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

console.log('PASS: All 11 test suites passed for StatusReportCard, DoD rich parsing, and theme tokens.')
