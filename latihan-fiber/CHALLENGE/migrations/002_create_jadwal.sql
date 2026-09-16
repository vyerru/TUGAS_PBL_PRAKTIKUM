CREATE TABLE IF NOT EXISTS jadwal (
    idjawal SERIAL PRIMARY KEY,
    idstudent SERIAL NOT NULL,
    mata_kuliah VARCHAR(100) NOT NULL,
    hari DATE NOT NULL,
    CONSTRAINT FK_students
    FOREIGN KEY(idstudent)
    REFERENCES students(id)
);