-- Migration 007: Dosen Pembimbing dan Penetapan Tugas Akhir
-- Kasus: Penetapan Dosen Pembimbing untuk Entity students (Tugas Mandiri D.2)

CREATE TABLE IF NOT EXISTS supervisors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    quota INTEGER NOT NULL CHECK (quota > 0),
    supervised_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Jaring pengaman terakhir: jumlah bimbingan tidak boleh melebihi kuota
    CONSTRAINT supervisors_supervised_within_quota
        CHECK (supervised_count >= 0 AND supervised_count <= quota)
);

CREATE TABLE IF NOT EXISTS supervisions (
    id BIGSERIAL PRIMARY KEY,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    supervisor_id INTEGER NOT NULL REFERENCES supervisors(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Seorang mahasiswa hanya boleh memiliki SATU pembimbing di level database:
    CONSTRAINT supervisions_student_unique UNIQUE (student_id)
);

CREATE INDEX IF NOT EXISTS supervisions_supervisor_idx ON supervisions (supervisor_id);

-- Permission baru: supervision:manage
INSERT INTO permissions (name, description) VALUES
('supervision:manage', 'Menetapkan dan mengelola dosen pembimbing tugas akhir')
ON CONFLICT (name) DO NOTHING;

-- Berikan permission supervision:manage ke role admin dan staff
INSERT INTO role_permissions (role_name, permission_name) VALUES
('admin', 'supervision:manage'),
('staff', 'supervision:manage')
ON CONFLICT DO NOTHING;

-- Seed beberapa data pembimbing
-- ID 1: kuota 3 (digunakan untuk uji beban D.3)
-- ID 2: kuota 5 (digunakan untuk uji switch pembimbing)
-- ID 3: kuota 10
INSERT INTO supervisors (id, name, quota) VALUES
(1, 'Dr. Ir. Budi Santoso, M.Kom.', 3),
(2, 'Dr. Eng. Siti Aminah, S.T., M.T.', 5),
(3, 'Prof. Dr. Agus Raharjo, M.Cs.', 10)
ON CONFLICT (id) DO NOTHING;

-- Seed 60 data mahasiswa uji untuk beban penetapan serentak D.3
INSERT INTO students (nim, name, grade, is_active)
SELECT 
    '434241021' || LPAD(g::TEXT, 3, '0'),
    'Mahasiswa TA ' || LPAD(g::TEXT, 3, '0'),
    'A',
    TRUE
FROM generate_series(1, 60) AS g
ON CONFLICT (nim) DO NOTHING;
