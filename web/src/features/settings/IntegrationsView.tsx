import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

export function IntegrationsView() {
  const { pathname } = useLocation()
  const [copyState, setCopyState] = useState('')
  const snippet = `<script src="${window.location.origin}/chat/widget.js" data-product="website-anda" defer></script>`
  if (!pathname.endsWith('/live-chat'))
    return (
      <section className="rounded-lg border bg-card p-5">
        <p className="my-3 text-sm text-muted-foreground">
          Hubungkan website Anda dengan Cuan.
        </p>
        <Link
          to="/settings/integrations/live-chat"
          className="action-secondary"
        >
          Live Chat →
        </Link>
      </section>
    )
  return (
    <section
      className="min-w-0 max-w-3xl space-y-5"
      aria-label="Integrasi Live Chat"
    >
      <Link to="/settings/integrations" className="action-secondary">
        ← Integrations
      </Link>
      <header>
        <p className="mt-2 text-sm text-muted-foreground">
          Hubungkan live chat ke website. Pengunjung yang membuka chat otomatis
          masuk ke Contacts.
        </p>
      </header>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">1. Pasang widget</h2>
        <p className="text-sm text-muted-foreground">
          Salin kode sebelum penutup body website. Ganti website-anda dengan
          label produk atau website Anda.
        </p>
        <pre className="max-w-full overflow-x-auto rounded-lg bg-muted p-3 text-sm select-all">
          <code>{snippet}</code>
        </pre>
        <button
          className="action-secondary"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(snippet)
              setCopyState('Kode berhasil disalin.')
            } catch {
              setCopyState(
                'Belum dapat menyalin. Pilih kode di atas dan salin secara manual.',
              )
            }
          }}
        >
          Salin kode widget
        </button>
        {copyState && (
          <p role="status" className="text-sm">
            {copyState}
          </p>
        )}
      </section>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">
          2. Identitas pelanggan (opsional)
        </h2>
        <p className="text-sm text-muted-foreground">
          Tambahkan atribut data-email dan data-name dari pengguna yang login,
          atau panggil kode berikut setelah widget dimuat:
        </p>
        <pre className="max-w-full overflow-x-auto rounded-lg bg-muted p-3 text-sm select-all">
          <code>{`window.$dojoChat.identify({\n  email: 'customer@example.com',\n  name: 'Nama pelanggan'\n})`}</code>
        </pre>
        <p className="text-sm text-muted-foreground">
          Email ini diklaim oleh widget, belum terverifikasi. Email yang sama
          tidak menggabungkan riwayat kontak lain. Jangan gunakan sebagai
          autentikasi atau pemisahan akun pada browser bersama.
        </p>
      </section>
      <section className="space-y-3 rounded-lg border bg-card p-5">
        <h2 className="text-base font-semibold">3. Periksa pemasangan</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm">
          <li>Buka website dan tekan tombol Live Chat.</li>
          <li>Kirim pesan percobaan dari widget.</li>
          <li>
            Buka Contacts, pilih pengunjung tersebut, lalu Buka percakapan untuk
            membalas.
          </li>
        </ol>
        <p className="text-sm text-muted-foreground">
          Jika widget tidak muncul, periksa kode pemasangan dan apakah website
          mengizinkan script dari {window.location.origin}.
        </p>
        <Link to="/contacts" className="action-primary">
          Buka Contacts
        </Link>
      </section>
    </section>
  )
}
