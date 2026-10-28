package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"MODUL_10/app/model"
	"MODUL_10/app/repository"
	"MODUL_10/helper"
)

// PortfolioService memakai DUA database sekaligus: data mahasiswa dibaca dari
// PostgreSQL, portofolio disimpan di MongoDB. Service-lah satu-satunya tempat
// yang mengenal keduanya.
type PortfolioService struct {
	portfolios repository.PortfolioRepository
	students   repository.StudentRepository
	perms      *helper.PermissionSet
}

func NewPortfolioService(
	portfolios repository.PortfolioRepository,
	students repository.StudentRepository,
	perms *helper.PermissionSet,
) *PortfolioService {
	return &PortfolioService{portfolios: portfolios, students: students, perms: perms}
}

// studentOwned mengambil mahasiswa dari PostgreSQL sekaligus memastikan
// pemanggil berhak. Tidak ada FOREIGN KEY lintas database: keberadaan
// mahasiswa hanya bisa ditanyakan ke PostgreSQL.
func (s *PortfolioService) studentOwned(ctx context.Context, c *fiber.Ctx, anyPermission string) (model.Student, error) {
	current, ok := helper.CurrentUser(c)
	if !ok {
		return model.Student{}, helper.Unauthorized("belum terautentikasi")
	}
	studentID, valid := helper.ParamID(c)
	if !valid {
		return model.Student{}, helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	student, err := s.students.FindByID(ctx, studentID)
	if err != nil {
		return model.Student{}, translatePortfolioError(err, "mahasiswa")
	}
	if !CanAccessStudent(current, student.OwnerID, s.perms, anyPermission) {
		return model.Student{}, helper.Forbidden(
			"tidak berhak mengakses portofolio mahasiswa milik orang lain")
	}
	return student, nil
}

// ---------- GET /students/:id/portfolio ----------

func (s *PortfolioService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	if _, err := s.studentOwned(ctx, c, "student:read:any"); err != nil {
		return err
	}
	studentID, _ := helper.ParamID(c)

	p, err := s.portfolios.FindByStudent(ctx, studentID)
	if err != nil {
		return translatePortfolioError(err, "portofolio")
	}
	return helper.Success(c, fiber.StatusOK, "portofolio ditemukan", p)
}

// ---------- PUT /students/:id/portfolio ----------

func (s *PortfolioService) Put(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	if _, err := s.studentOwned(ctx, c, "student:update:any"); err != nil {
		return err
	}
	studentID, _ := helper.ParamID(c)

	var req model.PutPortfolioRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Headline = strings.TrimSpace(req.Headline)
	req.Bio = strings.TrimSpace(req.Bio)
	projects := make([]model.Project, 0, len(req.Projects))
	for _, pr := range req.Projects {
		pr.Title = strings.TrimSpace(pr.Title)
		pr.Link = strings.TrimSpace(pr.Link)
		pr.Technologies = NormalizeTechnologies(pr.Technologies)
		projects = append(projects, model.Project{
			Title: pr.Title, Technologies: pr.Technologies, Link: pr.Link, Year: pr.Year,
		})
	}
	certificates := make([]model.Certificate, 0, len(req.Certificates))
	for _, cr := range req.Certificates {
		cr.Name = strings.TrimSpace(cr.Name)
		cr.Issuer = strings.TrimSpace(cr.Issuer)
		certificates = append(certificates, model.Certificate{
			Name: cr.Name, Issuer: cr.Issuer, Date: cr.Date,
		})
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	p, err := s.portfolios.Replace(ctx, studentID, model.Portfolio{
		StudentID: studentID, Headline: req.Headline, Bio: req.Bio,
		Projects: projects, Certificates: certificates,
	})
	if err != nil {
		return translatePortfolioError(err, "portofolio")
	}
	return helper.Success(c, fiber.StatusOK, "portofolio berhasil disimpan", p)
}

// ---------- POST /students/:id/portfolio/projects ----------

func (s *PortfolioService) AddProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	if _, err := s.studentOwned(ctx, c, "student:update:any"); err != nil {
		return err
	}
	studentID, _ := helper.ParamID(c)

	var req model.ProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Link = strings.TrimSpace(req.Link)
	req.Technologies = NormalizeTechnologies(req.Technologies)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	p, err := s.portfolios.AddProject(ctx, studentID, model.Project{
		Title: req.Title, Technologies: req.Technologies, Link: req.Link, Year: req.Year,
	})
	if err != nil {
		return translatePortfolioError(err, "portofolio")
	}
	return helper.Created(c, "proyek berhasil ditambahkan", p,
		"/api/v1/students/"+strconv.Itoa(studentID)+"/portfolio")
}

// ---------- DELETE /students/:id/portfolio/projects/:projectId ----------

func (s *PortfolioService) DeleteProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	if _, err := s.studentOwned(ctx, c, "student:update:any"); err != nil {
		return err
	}
	studentID, _ := helper.ParamID(c)

	p, err := s.portfolios.DeleteProject(ctx, studentID, c.Params("projectId"))
	if err != nil {
		return translatePortfolioError(err, "portofolio")
	}
	return helper.Success(c, fiber.StatusOK, "proyek berhasil dihapus", p)
}

// ---------- GET /portfolios/stats/technologies ----------

func (s *PortfolioService) TechnologyStats(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	stats, err := s.portfolios.TechnologyStats(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.SuccessList(c, "statistik teknologi berhasil diambil", stats, &model.Meta{
		Total: len(stats),
	})
}

func translatePortfolioError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrInvalidID):
		return helper.BadRequest("id proyek tidak valid")
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrProjectLimit):
		return helper.NewError(fiber.StatusConflict, "PROJECT_LIMIT",
			"jumlah proyek sudah mencapai batas")
	case errors.Is(err, repository.ErrProjectNotFound):
		return helper.NotFound("proyek tidak ditemukan di dalam portofolio")
	default:
		return helper.Internal(err)
	}
}
