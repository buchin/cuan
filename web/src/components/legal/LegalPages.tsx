import { ArrowLeft, Shield, FileText, CheckCircle2 } from 'lucide-react'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'

interface LegalPageProps {
  onNavigate: (path: string) => void
}

export function TermsPage({ onNavigate }: LegalPageProps) {
  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col">
      <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-20">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onNavigate('/')}
              className="h-8 text-xs text-slate-300 border-slate-700 bg-slate-800/60 hover:bg-slate-700 gap-1.5"
            >
              <ArrowLeft className="h-3.5 w-3.5" /> Kembali
            </Button>
            <span className="font-bold text-white text-sm">Cuan — Syarat & Ketentuan (Terms of Service)</span>
          </div>
          <span className="text-xs text-slate-400 font-mono">v1.0 Apache-2.0</span>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-10 space-y-6 flex-1">
        <Card className="bg-slate-900/90 border-slate-800 text-slate-200">
          <CardHeader className="border-b border-slate-800/80 pb-4">
            <div className="flex items-center gap-2 text-emerald-400 text-xs font-semibold uppercase tracking-wider">
              <FileText className="h-4 w-4" />
              Ketentuan Penggunaan Mandiri
            </div>
            <CardTitle className="text-xl font-bold text-white mt-1">
              Syarat & Ketentuan Penggunaan Cuan
            </CardTitle>
            <p className="text-xs text-slate-400">Terakhir diperbarui: 16 Oktober 2026</p>
          </CardHeader>
          <CardContent className="pt-6 space-y-4 text-xs leading-relaxed text-slate-300">
            <div>
              <h4 className="font-semibold text-white text-sm mb-1">1. Lisensi Perangkat Lunak</h4>
              <p>
                Cuan dirilis di bawah lisensi terbuka <strong>Apache License 2.0</strong>. Anda bebas menggunakan, memodifikasi, mendistribusikan, dan menjalankan Cuan di server pribadi Anda secara mandiri (self-hosted).
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">2. Tanggung Jawab Pengelolaan Server</h4>
              <p>
                Sebagai pemilik instance mandiri, Anda bertanggung jawab penuh atas keamanan server, konfigurasi domain, firewall, dan konten dari script atau aplikasi web yang Anda deploy di Cuan Studio.
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">3. Anti-Judol & Larangan Konten Ilegal</h4>
              <p>
                Seluruh aktivitas sponsorship, backer, dan deployment yang terhubung dengan domain publik Cuan wajib mematuhi etika dan hukum yang berlaku. Cuan secara tegas melarang perjudian online (anti-judol), konten berbahaya, malware, dan aktivitas penipuan.
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">4. Ketiadaan Garansi (As-Is)</h4>
              <p>
                Perangkat lunak disediakan "sebagaimana adanya" (AS IS) tanpa jaminan apa pun, baik tersurat maupun tersirat. Pengembang tidak bertanggung jawab atas kehilangan data yang disebabkan kelalaian backup host pengguna.
              </p>
            </div>
          </CardContent>
        </Card>
      </main>
    </div>
  )
}

export function PrivacyPage({ onNavigate }: LegalPageProps) {
  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col">
      <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-20">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onNavigate('/')}
              className="h-8 text-xs text-slate-300 border-slate-700 bg-slate-800/60 hover:bg-slate-700 gap-1.5"
            >
              <ArrowLeft className="h-3.5 w-3.5" /> Kembali
            </Button>
            <span className="font-bold text-white text-sm">Cuan — Kebijakan Privasi (Privacy Policy)</span>
          </div>
          <span className="text-xs text-slate-400 font-mono">100% Self-Hosted</span>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-10 space-y-6 flex-1">
        <Card className="bg-slate-900/90 border-slate-800 text-slate-200">
          <CardHeader className="border-b border-slate-800/80 pb-4">
            <div className="flex items-center gap-2 text-emerald-400 text-xs font-semibold uppercase tracking-wider">
              <Shield className="h-4 w-4" />
              Privasi & Kedaulatan Data
            </div>
            <CardTitle className="text-xl font-bold text-white mt-1">
              Kebijakan Privasi Mandiri Cuan
            </CardTitle>
            <p className="text-xs text-slate-400">Terakhir diperbarui: 16 Oktober 2026</p>
          </CardHeader>
          <CardContent className="pt-6 space-y-4 text-xs leading-relaxed text-slate-300">
            <div className="p-3 bg-emerald-950/30 border border-emerald-500/30 rounded-xl flex items-start gap-2.5">
              <CheckCircle2 className="h-4 w-4 text-emerald-400 shrink-0 mt-0.5" />
              <div>
                <strong className="text-emerald-300">Zero Telemetry & Zero Third-Party Tracking</strong>
                <p className="text-slate-400 mt-0.5">
                  Cuan tidak mengumpulkan analitik rahasia, data penggunaan, atau mengirimkan data deployment Anda ke server pihak ketiga mana pun.
                </p>
              </div>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">1. Penyimpanan Data Lokal</h4>
              <p>
                Semua data proyek, repository, deployment, dan log tersimpan di dalam volume lokal PostgreSQL dan direktori server Anda sendiri (`data/deployments/`).
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">2. Data Waiting List & Sponsorship</h4>
              <p>
                Email yang didaftarkan pada Waiting List atau Sponsorship hanya digunakan untuk verifikasi status sponsor dan pengiriman konfirmasi transaksi melalui gateway resmi Dojo.
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white text-sm mb-1">3. Kedaulatan Penuh</h4>
              <p>
                Anda berhak menghapus seluruh data kapan saja dengan mematikan instance atau membersihkan database PostgreSQL tanpa ada salinan yang tertinggal di pihak ketiga.
              </p>
            </div>
          </CardContent>
        </Card>
      </main>
    </div>
  )
}
