# Progres pekerjaan

Pie kecil di samping judul percakapan atau nama project menunjukkan bagian tugas
yang selesai. Pie muncul setelah ada tugas selesai. Di Inbox, percakapan dengan
satu task tidak memakai pie karena statusnya sudah tersedia pada task card.

Ketuk atau klik pie untuk membuka rincian jumlah tugas. Di desktop, mengarahkan
pointer juga membuka rincian. Angka tersedia untuk pembaca layar dan tombol dapat
dibuka dengan Enter, ditutup dengan Escape atau ketukan di luar panel.

- Hanya tugas **Selesai** mengisi pie. Tugas yang masih direviu belum selesai.
- Tugas dibatalkan dikeluarkan dari pembagi, tetapi tetap tercantum dalam rincian.
- Semua tugas selesai tanpa pembatalan menjadi centang hijau lembut.
- Jika pekerjaan tersisa selesai tetapi ada pembatalan, indikator menjadi lingkaran
  penuh netral. Rinciannya mengatakan **Semua pekerjaan tersisa selesai** dan
  menunjukkan jumlah tugas dibatalkan.
- Progres 0%, tanpa tugas, atau tanpa data tidak menampilkan pie.
- **Perlu perhatian** tetap terlihat meski progres 0%. Status bekerja dan review
  juga tetap tersedia. Pembatalan ditampilkan sebagai dibatalkan, bukan kegagalan.
- Menambah atau membuka kembali pekerjaan dapat menurunkan progres.

Perhitungan memakai ringkasan seluruh tugas, termasuk yang belum dimuat dalam
halaman. Pie bukan perkiraan waktu atau persentase langkah agent.

Cuplikan Inbox merangkum laporan sistem dengan bahasa singkat. Rincian laporan
asli tetap tersedia dalam percakapan. Eksekusi yang dibatalkan bisa menyimpan hasil
parsial sebagai review; indikator memakai kondisi eksekusi terbaru agar pembatalan
tersebut tidak keliru ditampilkan sebagai tugas yang menunggu review.

Menu tindakan percakapan tersedia lewat hover desktop, tekan lama di mobile, atau
navigasi keyboard. Ketukan biasa tetap membuka percakapan; mengetuk pie hanya
membuka rincian progres.

Dalam chat, ekor kecil muncul pada bubble pertama dari pengirim yang sama.
Pergantian pengirim, hari, atau jeda lima menit memulai rangkaian baru. Preview
pekerjaan dan pesan yang hanya berisi lampiran tidak memakai ekor.

Execution progress always belongs to the latest attempt. Retrying a task clears
its current progress until that attempt reports or emits useful stdout. Earlier
reports remain in its history. A queued attempt never borrows a previous run's
“finished” message. Detail and list use the same progress source.

### Result identity when publication fails

The native runner retains the independently observed clean workspace commit even
when production verification fails. This identifies the saved result for release
reconciliation; it does not establish publication or task acceptance. Dirty or
unreadable workspaces do not gain this identity. The original failed attempt and
publication error remain visible, and a verified deployment receipt or independent
registered release observation is still required before acceptance.

### Legacy deployment helper protection

Native activation installs independent systemd pre-start gates for the API,
runner and versioned chat workers. After payload, ancestry, migration and idle
checks, activation authorizes the exact candidate and retained current release.
The gate rejects a legacy workspace helper attempting to start another release,
even if that helper rewrites its usual `release.conf` or chat service template.
A failed activation can still restart the authorized previous release.

The policy is under `maintenance/activation-policy/`; ordinary deployments update
it through the native helper. It is not a task acceptance receipt or a workaround
for missing release ancestry. An explicit operator rollback still uses native
activation. Initial migration from an unversioned legacy install keeps its existing
rollback behavior; these gates start with the next native-to-native activation.
This protects against accidental stale helpers, not arbitrary root commands from
workers with unrestricted sudo. Do not claim an OS security sandbox.

### One authority for Inbox acceptance and continuation

A successful native worker execution reaches Inbox review. The worker's DoD
assessment is retained as a diagnostic tied to its exact attempt; it cannot
accept the task or create a correction attempt. The Inbox coordinator compares
the original approved scope, saved checklist and independent evidence before
acceptance or a scoped continuation. This also prevents a detached older worker
from reintroducing obsolete cross-project requirements after an upgrade. Native
execution, reporting, repository, publication and cancellation gates remain in
force. Ordinary board tasks without an Inbox retain their existing evaluator.

### Reconcile a verified branch with an existing live outcome

A native attempt which passed its project checks but failed only independent
publication verification may receive one read-only current-outcome inspection.
Eligibility requires its exact retained clean commit and native branch-diff/check
record, a final implementation report, and a scoped coordinator assessment that
only verification remains. Failed checks, cancellation, a different error, missing
native evidence, or a deployment prohibition cannot use this path.

The inspection uses the separately observed currently running revision, not the
unpublished branch. Existing scope, ownership, unchanged-source, visual evidence
and fresh native-check gates still apply. It cannot edit, push, deploy, fabricate
ancestry, change the source attempt's failed status, or accept the task itself.
Its request identity uses the existing source-attempt budget; moving production
does not grant an unlimited new implementation/retry allowance.

### Recheck a repaired native assertion without resetting retries

When a current-outcome inspection fails an actual Go assertion, the coordinator
keeps that concrete check failure separate from the older branch's missing
publication receipt. Current support derives conservative repository paths from
one failing internal/pkg package and its reported test source files; ambiguous,
unsupported or infrastructure-only output grants no repair allowance.

Revalidation requires the exact authenticated native check record, a final
implementation report, a matching point-in-time repository observation, the
same scoped coordinator review, and a newly observed running commit that actually
changes the named failing files. A pre-check observation from an older runner is
not final scope verification or evidence that the failed attempt succeeded.
The new inspection uses a separate exact snapshot and must supply fresh native
checks, source-boundary verification and any required visual evidence. Original
scope, ownership, cancellation and publication restrictions still apply.

The existing single revalidation request identity is retained. An unchanged or
unrelated release, foreign owner, modified source observation, missing evidence,
or another deployment after that allowance is consumed cannot open another retry.
The failed attempt and unpublished original branch keep their recorded states.
Native runners now remeasure repository state after project checks even when
those checks fail, preserving any mutation rather than resetting the files.
