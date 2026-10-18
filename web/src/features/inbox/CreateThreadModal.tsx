import React, { useState } from 'react'
import { Project } from '../../api/types'
import { X, MessageSquarePlus, Bot, Loader2 } from 'lucide-react'

interface CreateThreadModalProps {
  isOpen: boolean
  onClose: () => void
  projects: Project[]
  onSubmit: (data: { title: string; message: string; project_id?: string }) => Promise<void>
}

export function CreateThreadModal({
  isOpen,
  onClose,
  projects,
  onSubmit,
}: CreateThreadModalProps) {
  const [title, setTitle] = useState('')
  const [message, setMessage] = useState('')
  const [projectId, setProjectId] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  const activeProjects = projects.filter((p) => !p.is_archived)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      setError('Judul thread harus diisi')
      return
    }
    if (!message.trim()) {
      setError('Pesan untuk Scrum Master harus diisi')
      return
    }

    setLoading(true)
    setError(null)
    try {
      await onSubmit({
        title: title.trim(),
        message: message.trim(),
        project_id: projectId || (activeProjects.length > 0 ? activeProjects[0].id : undefined),
      })
      onClose()
      setTitle('')
      setMessage('')
      setProjectId('')
    } catch (err: any) {
      setError(err.message || 'Gagal membuat thread baru')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
      <div className="w-full max-w-lg bg-card text-card-foreground border border-border shadow-2xl rounded-lg p-6">
        <div className="flex items-center justify-between pb-4 border-b border-border mb-4">
          <div className="flex items-center gap-2">
            <div className="h-7 w-7 rounded bg-primary/10 text-primary flex items-center justify-center">
              <MessageSquarePlus className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">
                Diskusi Baru dengan Scrum Master
              </h2>
              <p className="text-[11px] text-muted-foreground">
                Sampaikan ide atau masalah; SM akan menyusun Definition of Done (DoD).
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground transition-colors p-1 rounded"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 text-destructive text-xs rounded">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">
              Judul Pekerjaan / Thread
            </label>
            <input
              type="text"
              required
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Contoh: Perbaiki responsivitas modal di tablet..."
              className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-hidden focus:ring-1 focus:ring-ring"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">
              Tautkan ke Project (Opsional)
            </label>
            <select
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-hidden focus:ring-1 focus:ring-ring"
            >
              <option value="">Pilih project otomatis...</option>
              {activeProjects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.project_type || 'custom'})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">
              Deskripsi / Pesan Pembuka
            </label>
            <textarea
              required
              rows={5}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="Jelaskan kebutuhan, masalah bug, atau outcome yang diharapkan secara natural..."
              className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-hidden focus:ring-1 focus:ring-ring resize-none font-mono"
            />
          </div>

          <div className="pt-2 flex items-center justify-end gap-2 border-t border-border">
            <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="px-3 py-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md disabled:opacity-50"
            >
              {loading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Menganalisis...</span>
                </>
              ) : (
                <>
                  <Bot className="h-3.5 w-3.5" />
                  <span>Kirim ke Scrum Master</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
