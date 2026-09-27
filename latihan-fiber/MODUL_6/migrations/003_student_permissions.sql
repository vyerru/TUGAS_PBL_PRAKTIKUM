INSERT INTO permissions (name, description) VALUES
('student:list', 'Melihat daftar seluruh student'),
('student:read:any', 'Melihat data student mana pun'),
('student:create', 'Menambahkan data student baru'),
('student:update:any', 'Mengubah data student mana pun'),
('student:delete', 'Menghapus data student')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name) VALUES
('admin', 'student:list'),
('admin', 'student:read:any'),
('admin', 'student:create'),
('admin', 'student:update:any'),
('admin', 'student:delete'),
('staff', 'student:list'),
('staff', 'student:read:any'),
('staff', 'student:create')
ON CONFLICT DO NOTHING;

ALTER TABLE students ADD COLUMN IF NOT EXISTS owner_id INTEGER;

UPDATE students 
SET owner_id = (SELECT id FROM users ORDER BY id ASC LIMIT 1) 
WHERE owner_id IS NULL;

ALTER TABLE students 
ADD CONSTRAINT students_owner_id_fkey 
FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE;