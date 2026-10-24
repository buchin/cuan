-- Canned Responses (Saved Replies) for operator live chat
CREATE TABLE IF NOT EXISTS canned_responses (
    id UUID PRIMARY KEY,
    shortcut TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS canned_responses_shortcut_uidx ON canned_responses(lower(shortcut));
CREATE INDEX IF NOT EXISTS canned_responses_updated_idx ON canned_responses(updated_at DESC);
CREATE INDEX IF NOT EXISTS canned_responses_category_idx ON canned_responses(category);

INSERT INTO canned_responses (id, shortcut, title, content, category, created_at, updated_at) VALUES
('00000000-0000-4000-a000-000000000001', 'halo', 'Salam Pembuka', 'Halo! Terima kasih telah menghubungi kami. Ada yang bisa kami bantu hari ini?', 'greeting', NOW(), NOW()),
('00000000-0000-4000-a000-000000000002', 'tunggu', 'Mohon Menunggu', 'Mohon tunggu sebentar ya, saya sedang mengecek informasi terkait hal ini.', 'general', NOW(), NOW()),
('00000000-0000-4000-a000-000000000003', 'harga', 'Informasi Paket & Harga', 'Informasi paket langganan dan fitur lengkap dapat dilihat di website resmi kami atau kami bantu jelaskan di sini jika ada paket tertentu yang Anda minati.', 'sales', NOW(), NOW()),
('00000000-0000-4000-a000-000000000004', 'terimakasih', 'Penutup & Terima Kasih', 'Sama-sama! Senang bisa membantu Anda. Jika ada pertanyaan lain, jangan ragu untuk menghubungi kami kembali. Semoga harimu menyenangkan!', 'closing', NOW(), NOW())
ON CONFLICT (lower(shortcut)) DO NOTHING;
