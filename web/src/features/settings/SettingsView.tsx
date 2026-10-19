import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import type { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import { IntegrationsView } from './IntegrationsView'
import { TeamView } from '../team/TeamView'

interface SettingsViewProps {
  activeProject?: Project
  projects?: Project[]
  onAgentAdded?: () => void
  onDeleteProject?: (id: string) => Promise<void>
}
export function SettingsView({
  activeProject,
  projects = [],
  onAgentAdded,
  onDeleteProject,
}: SettingsViewProps) {
  const location = useLocation()
  const { theme, setTheme } = useTheme()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const section = location.pathname.split('/')[2] || 'general'
  const query = new URLSearchParams(location.search)
  const returnTo = query.get('returnTo') || ''
  const safeReturn = /^\/(inbox)(\/[^?#]*)?$/.test(returnTo) ? returnTo : ''
  if (activeProject)
    return (
      <section className="page-content max-w-3xl space-y-6">
        <Link
          to={`/projects/${encodeURIComponent(activeProject.id)}`}
          className="action-secondary md:hidden"
        >
          ← Kembali ke project
        </Link>
        <header>
          <p className="mt-2 text-sm text-muted-foreground">
            Pengaturan khusus project {activeProject.name}.
          </p>
        </header>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {notice && (
          <p role="status" className="text-sm">
            {notice}
          </p>
        )}
        <section className="space-y-4 rounded-lg border border-border bg-card p-5">
          <h2 className="text-lg font-semibold">Repository</h2>
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="font-medium">Repository URL</dt>
              <dd className="mt-1 break-all text-muted-foreground">
                {activeProject.repo_url || 'Belum terhubung'}
              </dd>
            </div>
            <div>
              <dt className="font-medium">Default branch</dt>
              <dd className="mt-1 text-muted-foreground">
                {activeProject.default_branch || 'Belum tersedia'}
              </dd>
            </div>
          </dl>
          <p className="text-sm text-muted-foreground">
            Detail ini mengikuti konfigurasi project tersimpan. Pengubahan
            koneksi repository belum tersedia di halaman ini.
          </p>
        </section>
        {onDeleteProject && !activeProject.is_archived && (
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">Arsipkan project</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Project dipindahkan ke daftar arsip dan dapat dipulihkan dari
              Projects. Repository dan data tetap disimpan.
            </p>
            <button
              disabled={busy}
              className="action-secondary"
              onClick={async () => {
                if (!confirm(`Arsipkan project "${activeProject.name}"?`))
                  return
                setBusy(true)
                setError('')
                try {
                  await onDeleteProject(activeProject.id)
                  setNotice('Project telah diarsipkan.')
                } catch {
                  setError('Project belum berhasil diarsipkan. Coba lagi.')
                } finally {
                  setBusy(false)
                }
              }}
            >
              Arsipkan project
            </button>
          </section>
        )}
      </section>
    )
  return (
    <section className="page-content space-y-5">
      <header>
        <p className="mt-1 text-sm text-muted-foreground">
          Kelola aplikasi dan koneksi agen Anda.
        </p>
      </header>
      <nav
        aria-label="Bagian pengaturan"
        className="flex gap-1 overflow-x-auto border-b border-border"
      >
        {[
          ['general', 'Umum'],
          ['agents', 'Agents'],
          ['integrations', 'Integrations'],
          ['appearance', 'Tampilan'],
        ].map(([id, label]) => (
          <Link
            key={id}
            to={id === 'general' ? '/settings' : `/settings/${id}`}
            aria-current={section === id ? 'page' : undefined}
            className={`inline-flex min-h-11 shrink-0 items-center border-b-2 px-3 text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${section === id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
          >
            {label}
          </Link>
        ))}
      </nav>
      {section === 'agents' ? (
        <>
          {safeReturn && (
            <Link to={safeReturn} className="action-secondary">
              ← Kembali ke percakapan
            </Link>
          )}
          <TeamView
            showHeading={false}
            focusAgentId={query.get('agent') || undefined}
            onAgentAdded={onAgentAdded}
          />
        </>
      ) : section === 'integrations' ? (
        <IntegrationsView />
      ) : section === 'appearance' ? (
        <section className="rounded-lg border border-border bg-card p-5">
          <h2 className="text-lg font-semibold">Tema aplikasi</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Muse mengikuti pilihan Anda. Perubahan tersimpan di browser ini.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            {(['light', 'dark', 'system'] as const).map((id) => (
              <button
                key={id}
                type="button"
                aria-pressed={theme === id}
                onClick={() => setTheme(id)}
                className={`action-secondary ${theme === id ? 'bg-accent font-semibold ring-2 ring-ring' : ''}`}
              >
                {{ light: 'Light', dark: 'Dark', system: 'Ikuti sistem' }[id]}
              </button>
            ))}
          </div>
        </section>
      ) : (
        <div className="space-y-4">
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">Agents</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Tambahkan agen atau periksa koneksi akun yang sudah terhubung.
            </p>
            <Link to="/settings/agents" className="action-primary">
              Kelola agen
            </Link>
          </section>
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">Pengaturan project</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Pilih project untuk mengubah konfigurasi repository.
            </p>
            <ul className="space-y-2">
              {projects
                .filter((p) => !p.is_archived)
                .map((p) => (
                  <li key={p.id}>
                    <Link
                      to={`/projects/${encodeURIComponent(p.id)}/settings`}
                      className="action-secondary"
                    >
                      {p.name}
                    </Link>
                  </li>
                ))}
            </ul>
            {!projects.some((p) => !p.is_archived) && (
              <Link to="/projects" className="action-secondary">
                Buka Projects
              </Link>
            )}
          </section>
        </div>
      )}
    </section>
  )
}
