-- reset_supervisors.sql: mengembalikan keadaan data pembimbing dan penetapan TA
DELETE FROM supervisions;
UPDATE supervisors s
SET supervised_count = (SELECT COUNT(*) FROM supervisions sv WHERE sv.supervisor_id = s.id);
