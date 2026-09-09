package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"MODUL_3/app/model"
	"MODUL_3/app/repository"
)

type StudentHandler struct {
	repo repository.StudentRepository
}

// NewStudentHandler menerima INTERFACE, bukan struct konkret.
// Handler tidak tahu dan tidak perlu tahu datanya disimpan di mana.
func NewStudentHandler(repo repository.StudentRepository) *StudentHandler {
	return &StudentHandler{repo: repo}
}

// terjemahkanErrorStudent memetakan error repository menjadi status HTTP.
func terjemahkanErrorStudent(c *fiber.Ctx, err error, pesanUmum string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return fail(c, fiber.StatusNotFound, "student tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return fail(c, fiber.StatusConflict, "nim sudah dipakai")
	default:
		return fail(c, fiber.StatusInternalServerError, pesanUmum)
	}
}

func (h *StudentHandler) List(c *fiber.Ctx) error {
	ctx, cancel := reqCtx(c)
	defer cancel()

	q := parseListQuery(c)

	students, total, err := h.repo.FindAll(ctx, q)
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "gagal mengambil data student")
	}

	totalPages := 0
	if q.Limit > 0 {
		totalPages = (total + q.Limit - 1) / q.Limit
	}

	return okList(c, "daftar student berhasil diambil", students, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

func (h *StudentHandler) Get(c *fiber.Ctx) error {
	ctx, cancel := reqCtx(c)
	defer cancel()

	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	student, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return terjemahkanErrorStudent(c, err, "gagal mengambil data student")
	}
	return ok(c, "student ditemukan", student)
}

func (h *StudentHandler) Create(c *fiber.Ctx) error {
	ctx, cancel := reqCtx(c)
	defer cancel()

	var req model.CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)
	req.Grade = strings.TrimSpace(req.Grade)

	errs := map[string]string{}
	if req.NIM == "" {
		errs["nim"] = "wajib diisi"
	}
	if req.Name == "" {
		errs["name"] = "wajib diisi"
	}
	if req.Grade == "" {
		errs["grade"] = "wajib diisi"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	// Keunikan NIM TIDAK diperiksa dengan SELECT lebih dulu — dijamin
	// oleh UNIQUE INDEX di basis data untuk menghindari race condition.
	baru, err := h.repo.Create(ctx, model.Student{
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: true,
	})
	if err != nil {
		return terjemahkanErrorStudent(c, err, "gagal menyimpan student")
	}

	return created(c, "student berhasil dibuat", baru,
		"/api/v1/students/"+strconv.Itoa(baru.ID))
}