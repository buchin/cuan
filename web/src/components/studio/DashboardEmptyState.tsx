import { Card, CardContent } from '../ui/card'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  FolderPlus,
  Layers,
  Globe,
  Bot,
  ArrowRight
} from 'lucide-react'

interface DashboardEmptyStateProps {
  onCreateProjectClick: () => void
  onExploreGlobalClick?: () => void
}

export function DashboardEmptyState({
  onCreateProjectClick,
  onExploreGlobalClick,
}: DashboardEmptyStateProps) {
  return (
    <div className="w-full max-w-5xl mx-auto py-8 sm:py-12 px-4 space-y-8 animate-in fade-in-50">
      {/* Hero Welcome Card */}
      <div className="relative overflow-hidden rounded-2xl border border-primary/20 bg-gradient-to-b from-card via-card/90 to-background p-8 sm:p-12 text-center shadow-2xl">
        {/* Subtle background glow */}
        <div className="absolute -top-24 left-1/2 -translate-x-1/2 w-96 h-96 bg-primary/10 rounded-full blur-3xl pointer-events-none" />

        <div className="relative z-10 max-w-2xl mx-auto space-y-4">
          <Badge className="bg-primary/10 text-primary border-primary/30 text-xs py-1 px-3 font-semibold uppercase tracking-wider">
            Workspace Onboarding
          </Badge>

          <h1 className="text-2xl sm:text-4xl font-extrabold tracking-tight text-foreground">
            Mulai Perjalanan Anda dengan{' '}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-primary via-indigo-400 to-purple-400">
              Proyek Pertama
            </span>
          </h1>

          <p className="text-sm sm:text-base text-muted-foreground leading-relaxed">
            Di Cuan, setiap proyek adalah workspace terpadu yang menghubungkan Git repository,
            deployment kontainer, cloud WordPress, AI autonomous agents, dan Kanban board multi-proyek.
          </p>

          <div className="pt-4 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Button
              onClick={onCreateProjectClick}
              size="lg"
              className="w-full sm:w-auto h-12 px-6 bg-primary text-primary-foreground hover:bg-primary/90 font-bold text-sm shadow-xl shadow-primary/20 gap-2.5 active:scale-95 transition-all"
            >
              <FolderPlus className="h-5 w-5" />
              Create New Project
            </Button>

            {onExploreGlobalClick && (
              <Button
                variant="outline"
                size="lg"
                onClick={onExploreGlobalClick}
                className="w-full sm:w-auto h-12 px-5 text-xs text-muted-foreground hover:text-foreground border-border bg-card/40 hover:bg-muted"
              >
                <span>Lihat Navigasi Global</span>
                <ArrowRight className="h-4 w-4 ml-1.5" />
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* 3 Key Educational Pillars */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        <Card className="bg-card/70 border-border/80 hover:border-primary/40 transition-all hover:shadow-lg">
          <CardContent className="p-6 space-y-3">
            <div className="flex h-10 w-10 items-center justify-center bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
              <Layers className="h-5 w-5" />
            </div>
            <h2 className="text-sm font-bold text-foreground">
              Modul Proyek Kontekstual
            </h2>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Setiap kali Anda membuka proyek, sidebar bertransformasi menyajikan sub-modul kontekstual:
              Overview, WordPress Cloud, Repos, Deployments, Telemetry, Studio Rooms, dan Settings.
            </p>
          </CardContent>
        </Card>

        <Card className="bg-card/70 border-border/80 hover:border-primary/40 transition-all hover:shadow-lg">
          <CardContent className="p-6 space-y-3">
            <div className="flex h-10 w-10 items-center justify-center bg-purple-500/10 text-purple-400 border border-purple-500/20">
              <Bot className="h-5 w-5" />
            </div>
            <h2 className="text-sm font-bold text-foreground">
              Autonomous AI Agents & Rooms
            </h2>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Delegasikan pekerjaan teknis kepada agent (Dev Team, Webmaster, Researcher) yang mengeksekusi
              task secara mandiri dengan verifikasi DoD otomatis dan sinkronisasi log real-time.
            </p>
          </CardContent>
        </Card>

        <Card className="bg-card/70 border-border/80 hover:border-primary/40 transition-all hover:shadow-lg">
          <CardContent className="p-6 space-y-3">
            <div className="flex h-10 w-10 items-center justify-center bg-blue-500/10 text-blue-400 border border-blue-500/20">
              <Globe className="h-5 w-5" />
            </div>
            <h2 className="text-sm font-bold text-foreground">
              Turnkey Cloud & Multi-Stack
            </h2>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Jalankan WordPress Turnkey, microservices Docker, static landing pages, atau aplikasi full-stack
              dengan alokasi port terisolasi dan custom domain Caddy TLS otomatis.
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Quick Setup Checklist */}
      <div className="rounded-xl border border-border bg-card/40 p-5 sm:p-6">
        <h3 className="text-xs font-bold uppercase tracking-wider text-muted-foreground mb-4">
          Langkah Mudah Memulai Workspace
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
          <div className="flex items-start gap-3">
            <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary font-bold text-xs border border-primary/30">
              1
            </div>
            <div>
              <span className="font-semibold text-foreground block">Buat Proyek</span>
              <span className="text-muted-foreground text-[11px]">
                Tentukan nama proyek, tipe aplikasi, dan modul yang ingin diaktifkan.
              </span>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary font-bold text-xs border border-primary/30">
              2
            </div>
            <div>
              <span className="font-semibold text-foreground block">Hubungkan Repositori</span>
              <span className="text-muted-foreground text-[11px]">
                Kaitkan dengan GitHub repo atau deploy script langsung dari platform.
              </span>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary font-bold text-xs border border-primary/30">
              3
            </div>
            <div>
              <span className="font-semibold text-foreground block">Beralih 1-Klik</span>
              <span className="text-muted-foreground text-[11px]">
                Gunakan Breadcrumb Switcher di navbar atas untuk berpindah workspace kapan saja.
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
