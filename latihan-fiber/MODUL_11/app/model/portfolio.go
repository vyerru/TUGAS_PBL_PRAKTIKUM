package model

import "time"

// Portfolio adalah bentuk portofolio mahasiswa yang dikenal seluruh aplikasi.
//
// Sama seperti Review pada Langkah 4: tidak ada satu pun tipe dari driver
// MongoDB di sini. Data mahasiswa (nim, nama) tetap di PostgreSQL; MongoDB
// hanya menyimpan portofolionya. Setiap mahasiswa memiliki TEPAT SATU
// dokumen portofolio — aturan ini dijaga index unik di repository.
type Portfolio struct {
	ID           string       `json:"id"`
	StudentID    int          `json:"student_id"`
	Headline     string       `json:"headline"`
	Bio          string       `json:"bio"`
	Projects     []Project    `json:"projects"`
	Certificates []Certificate `json:"certificates"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// Project disimpan DI DALAM dokumen Portfolio (embedding). Jumlahnya dibatasi
// 20 dan selalu dibaca bersama portofolio induknya — dua syarat embedding.
type Project struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Link         string   `json:"link"`
	Year         int      `json:"year"`
}

// Certificate juga embedded. Tidak butuh id karena tidak ada endpoint
// yang menghapus satu sertifikat sendirian.
type Certificate struct {
	Name   string    `json:"name"`
	Issuer string    `json:"issuer"`
	Date   time.Time `json:"date"`
}

// PutPortfolioRequest adalah body PUT /students/:id/portfolio: membuat ATAU
// mengganti seluruh portofolio sekaligus.
type PutPortfolioRequest struct {
	Headline     string               `json:"headline"  validate:"required,min=3,max=120"`
	Bio          string               `json:"bio"       validate:"max=2000"`
	Projects     []ProjectRequest     `json:"projects"     validate:"max=20,dive"`
	Certificates []CertificateRequest `json:"certificates" validate:"max=50,dive"`
}

// ProjectRequest dipakai dua tempat: di dalam PutPortfolioRequest dan sebagai
// body POST /students/:id/portfolio/projects.
type ProjectRequest struct {
	Title        string   `json:"title" validate:"required,min=3,max=100"`
	Technologies []string `json:"technologies" validate:"required,max=10,dive,min=2,max=30"`
	Link         string   `json:"link" validate:"omitempty,url,max=200"`
	Year         int      `json:"year" validate:"required,min=1990,max=2100"`
}

type CertificateRequest struct {
	Name   string    `json:"name"   validate:"required,min=3,max=100"`
	Issuer string    `json:"issuer" validate:"required,min=2,max=100"`
	Date   time.Time `json:"date"   validate:"required"`
}

// TechnologyStat adalah satu baris hasil
// GET /portfolios/stats/technologies.
type TechnologyStat struct {
	Technology string `json:"technology"`
	Total      int    `json:"total"`
}
