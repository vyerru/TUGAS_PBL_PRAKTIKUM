package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"MODUL_10/app/model"
)

// MaxProjectsPerPortfolio membatasi pertumbuhan array projects — pola yang
// sama dengan MaxRepliesPerReview. Satu dokumen MongoDB maksimal 16 MB.
const MaxProjectsPerPortfolio = 20

var ErrProjectLimit = errors.New("jumlah proyek sudah mencapai batas")
var ErrProjectNotFound = errors.New("proyek tidak ditemukan")

type PortfolioRepository interface {
	EnsureIndexes(ctx context.Context) error
	FindByStudent(ctx context.Context, studentID int) (model.Portfolio, error)
	Replace(ctx context.Context, studentID int, p model.Portfolio) (model.Portfolio, error)
	AddProject(ctx context.Context, studentID int, p model.Project) (model.Portfolio, error)
}

// portfolioDocument adalah bentuk portofolio SEBAGAIMANA TERSIMPAN di MongoDB.
// Tag bson menentukan nama field di database; struct ini tidak pernah dikirim
// ke client, sehingga package model tetap bebas dari tipe milik driver.
type portfolioDocument struct {
	ID           bson.ObjectID        `bson:"_id,omitempty"`
	StudentID    int                  `bson:"student_id"`
	Headline     string               `bson:"headline"`
	Bio          string               `bson:"bio"`
	Projects     []projectDocument    `bson:"projects"`
	Certificates []certificateDocument `bson:"certificates"`
	CreatedAt    time.Time            `bson:"created_at"`
	UpdatedAt    time.Time            `bson:"updated_at"`
}

type projectDocument struct {
	ID           bson.ObjectID `bson:"_id"`
	Title        string        `bson:"title"`
	Technologies []string      `bson:"technologies"`
	Link         string        `bson:"link,omitempty"`
	Year         int           `bson:"year"`
}

type certificateDocument struct {
	Name   string    `bson:"name"`
	Issuer string    `bson:"issuer"`
	Date   time.Time `bson:"date"`
}

func toProjectDocuments(in []model.Project) []projectDocument {
	out := make([]projectDocument, 0, len(in))
	for _, p := range in {
		tech := p.Technologies
		if tech == nil {
			tech = []string{}
		}
		out = append(out, projectDocument{
			ID: bson.NewObjectID(), Title: p.Title, Technologies: tech,
			Link: p.Link, Year: p.Year,
		})
	}
	return out
}

func toCertificateDocuments(in []model.Certificate) []certificateDocument {
	out := make([]certificateDocument, 0, len(in))
	for _, c := range in {
		out = append(out, certificateDocument{Name: c.Name, Issuer: c.Issuer, Date: c.Date})
	}
	return out
}

func toProjects(in []projectDocument) []model.Project {
	out := make([]model.Project, 0, len(in))
	for _, p := range in {
		tech := p.Technologies
		if tech == nil {
			tech = []string{}
		}
		out = append(out, model.Project{
			ID: p.ID.Hex(), Title: p.Title, Technologies: tech,
			Link: p.Link, Year: p.Year,
		})
	}
	return out
}

func toCertificates(in []certificateDocument) []model.Certificate {
	out := make([]model.Certificate, 0, len(in))
	for _, c := range in {
		out = append(out, model.Certificate{Name: c.Name, Issuer: c.Issuer, Date: c.Date})
	}
	return out
}

func (d portfolioDocument) toModel() model.Portfolio {
	return model.Portfolio{
		ID: d.ID.Hex(), StudentID: d.StudentID, Headline: d.Headline, Bio: d.Bio,
		Projects: toProjects(d.Projects), Certificates: toCertificates(d.Certificates),
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type portfolioMongoRepository struct{ coll *mongo.Collection }

func NewPortfolioRepository(db *mongo.Database) PortfolioRepository {
	return &portfolioMongoRepository{coll: db.Collection("student_portfolios")}
}

// EnsureIndexes dijalankan sekali saat aplikasi menyala.
//
// HANYA SATU index dibutuhkan, dan alasannya disengaja:
//   - {student_id: 1} unique — menjaga "satu mahasiswa satu portofolio" di
//     level database, dan sekaligus melayani seluruh endpoint yang selalu
//     mengakses portofolio lewat student_id.
//   - Tidak ada endpoint yang memfilter proyek per teknologi; stats membaca
//     SELURUH collection (aggregation tanpa $match), sehingga index selective
//     apa pun tidak akan dipakai di sana. Index yang tidak dipakai hanya
//     menambah biaya tulis.
func (r *portfolioMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "student_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("student_portfolio_unique"),
		},
	})
	if err != nil {
		return fmt.Errorf("membuat index student_portfolios: %w", err)
	}
	return nil
}

func (r *portfolioMongoRepository) FindByStudent(ctx context.Context, studentID int) (model.Portfolio, error) {
	var doc portfolioDocument
	err := r.coll.FindOne(ctx, bson.M{"student_id": studentID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.Portfolio{}, ErrNotFound
	}
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("mengambil portofolio: %w", err)
	}
	return doc.toModel(), nil
}

// Replace membuat ATAU mengganti seluruh portofolio (semantik PUT).
//
// created_at dan _id portofolio lama dipertahankan bila portofolionya sudah
// ada, sehingga mengganti isi tidak mengubah identitas dokumennya. Bila dua
// request PUT serentak sama-sama mendapati portofolio belum ada, index unik
// menolak yang kedua — percobaan mengganti diulang setelah pembacaan ulang.
func (r *portfolioMongoRepository) Replace(ctx context.Context, studentID int, p model.Portfolio) (model.Portfolio, error) {
	for attempt := 0; attempt < 3; attempt++ {
		doc := portfolioDocument{
			StudentID:    studentID,
			Headline:     p.Headline,
			Bio:          p.Bio,
			Projects:     toProjectDocuments(p.Projects),
			Certificates: toCertificateDocuments(p.Certificates),
			CreatedAt:    now(),
			UpdatedAt:    now(),
		}

		var existing portfolioDocument
		err := r.coll.FindOne(ctx, bson.M{"student_id": studentID}).Decode(&existing)
		switch {
		case err == nil:
			doc.ID = existing.ID
			doc.CreatedAt = existing.CreatedAt
		case errors.Is(err, mongo.ErrNoDocuments):
			doc.ID = bson.NewObjectID()
		default:
			return model.Portfolio{}, fmt.Errorf("memeriksa portofolio: %w", err)
		}

		opts := options.FindOneAndReplace().
			SetUpsert(true).
			SetReturnDocument(options.After)

		err = r.coll.FindOneAndReplace(ctx, bson.M{"student_id": studentID}, doc, opts).Decode(&doc)
		if err == nil {
			return doc.toModel(), nil
		}
		if mongo.IsDuplicateKeyError(err) {
			// Portofolio dibuat proses lain di sela-sela pembacaan kita.
			// Ulangi: pembacaan berikutnya pasti menemukan dokumennya.
			continue
		}
		return model.Portfolio{}, fmt.Errorf("menyimpan portofolio: %w", err)
	}
	return model.Portfolio{}, fmt.Errorf("menyimpan portofolio: terlalu banyak percobaan bersamaan")
}

// AddProject menambahkan satu proyek ke array projects.
//
// Syarat "projects.19 tidak ada" berarti array belum berisi 20 elemen
// (indeks 0 sampai 19). Pemeriksaan batas dan penambahan terjadi dalam satu
// operasi atomik, sehingga dua request serentak tidak dapat sama-sama lolos
// ketika tinggal satu tempat tersisa — pola yang sama dengan AddReply.
func (r *portfolioMongoRepository) AddProject(ctx context.Context, studentID int, p model.Project) (model.Portfolio, error) {
	limitField := "projects." + strconv.Itoa(MaxProjectsPerPortfolio-1)

	doc := projectDocument{
		ID: bson.NewObjectID(), Title: p.Title,
		Technologies: p.Technologies, Link: p.Link, Year: p.Year,
	}
	if doc.Technologies == nil {
		doc.Technologies = []string{}
	}

	var updated portfolioDocument
	err := r.coll.FindOneAndUpdate(ctx,
		bson.M{"student_id": studentID, limitField: bson.M{"$exists": false}},
		bson.M{
			"$push":  bson.M{"projects": doc},
			"$set":   bson.M{"updated_at": now()},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Portfolio{}, fmt.Errorf("menambah proyek: %w", err)
	}

	count, err := r.coll.CountDocuments(ctx, bson.M{"student_id": studentID})
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("memeriksa portofolio: %w", err)
	}
	if count == 0 {
		return model.Portfolio{}, ErrNotFound
	}
	return model.Portfolio{}, ErrProjectLimit
}
