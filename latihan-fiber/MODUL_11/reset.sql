-- Skrip reset untuk uji beban
-- Setiap uji beban mengubah isi database. Sebelum mengulang sebuah uji, kembalikan keadaannya dengan skrip berikut.
DELETE FROM enrollments
WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'mhs%');
UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);