# CRM Contacts dan konsistensi daftar

- Contacts: daftar satu kolom padat, maksimal2 chip, metadata sumber/update.
- Detail di HP/tablet terpisah; desktop>=1200px split. Query contact menyimpan
  pilihan saat deep link/reload. Aksi utama membuka percakapan.
- Editor tag memakai chip, saran tag, validasi40karakter/20tag dan explicit Save.
  Tidak ada penulisan sebelum Save; kegagalan mempertahankan draft.
- Filter sumber/tag/pencarian memiliki reset.
- Tasks reset project global, tetapi board project tetap terikat project asal.
- Projects mengingat preferensi Grid/List dan menawarkan clear search.
- Agents dapat dicari tanpa probe provider atau login otomatis.

Tidak ada perubahan schema/API. Email/nama tetap data widget (belum terverifikasi);
edit identitas manual dan broadcast tetap belum tersedia. Verifikasi memakai
fixture terisolasi pada build production nyata, termasuk panjang data, duplicate
case, batas tag, save failure/retry, empty tags, mobile safe-area, Back/deep link,
load error dan invalid contact. Chromium emulation, bukan sertifikasi Safari.
