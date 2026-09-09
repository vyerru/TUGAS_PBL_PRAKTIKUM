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