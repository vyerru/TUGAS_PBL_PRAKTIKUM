-- Migration 010: idempotency key notifikasi per PENERIMA
--   psql -U postgres -d praktikum_backend -f migrations/010_notifications_per_recipient.sql
--
-- Latar belakang
-- --------------
-- Sampai Modul 11, satu event outbox hanya pernah menghasilkan satu
-- notifikasi. Karena itu UNIQUE (outbox_id) cukup menjadi idempotency key.
--
-- D.2 mengubah hal itu: satu event course.quota_changed harus menghasilkan
-- satu notifikasi untuk SETIAP peserta kelas. Dengan UNIQUE (outbox_id),
-- notifikasi kedua untuk event yang sama ditolak, sehingga hanya peserta
-- pertama yang menerima email.
--
-- Pilihan key: UNIQUE (outbox_id, user_id)
-- ----------------------------------------
-- Pasangan ini menjawab pertanyaan yang sebenarnya: "apakah event INI sudah
-- dikirim ke ORANG INI?". Sifatnya tetap idempoten untuk task yang datang
-- berulang kali (at-least-once), tetapi tidak lagi membatasi satu event ke
-- satu penerima.
--
-- channel sengaja belum dimasukkan ke key. Saat ini semua notifikasi
-- memakai channel "email"; bila kelak ada kanal lain (push, SMS), key dapat
-- diperluas menjadi (outbox_id, user_id, channel) tanpa kehilangan makna.
-- Memasukkan channel sekarang hanya akan membuat key lebih lebar tanpa
-- menambah jaminan apa pun.

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_outbox_unique;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_outbox_user_unique UNIQUE (outbox_id, user_id);

-- Query worker selalu mencari berdasarkan pasangan (outbox_id, user_id)
-- melalui ON CONFLICT; constraint di atas sudah menyediakan index-nya.
-- Index tambahan ini membantu pemeriksaan "berapa penerima yang sudah
-- tercatat untuk satu event" saat menelusuri masalah.
CREATE INDEX IF NOT EXISTS notifications_outbox_idx
    ON notifications (outbox_id);
