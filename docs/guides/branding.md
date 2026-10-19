# Identitas Cuan — uang kepeng

Logo mengikuti referensi PO: uang kepeng berbentuk lingkaran dengan lubang
persegi dan cincin luar. Lubang persegi transparan, sehingga warna latar tetap
terlihat. Bentuk dipertahankan sama pada light dan dark; hanya warnanya berbalik.

## Penggunaan

- Logo inline aplikasi memakai foreground tema: gelap di light, terang di dark.
- Sidebar menggunakan mark40px dan wordmark `cuan`; sidebar lipat tetap menampilkan mark.
- Header HP menggunakan mark24px, login36px. Tidak menambah tombol navigasi.
- Favicon `/favicon.svg` mengikuti preferensi warna browser/OS, sementara logo
  dalam aplikasi mengikuti pilihan tema Settings. Keduanya dapat berbeda bila
  pengguna memilih override tema aplikasi.
- Export transparan: `/brand/cuan-light.svg` untuk latar terang dan
  `/brand/cuan-dark.svg` untuk latar gelap. Gunakan tanpa merentangkan proporsi.

SVG menjaga ketajaman di ukuran kecil. Bukti visual16/24/32/64/128px, login,
sidebar dan HP tersedia pada `docs/evidence/kepeng-brand/`. Pengujian viewport
Chromium tidak menggantikan pemeriksaan semua perangkat/browser fisik.

Rilis `252c36d` aktif di https://cuan.dojo.cc. Production memverifikasi delapan
pasangan theme/viewport, login light/dark, favicon serta export SVG identik
dengan build. Schema22 tetap dan data/profil lama utuh. Bukti:
`docs/evidence/kepeng-brand/production-browser.json`.
