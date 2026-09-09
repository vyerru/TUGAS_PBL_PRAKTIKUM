// handler.go — bagian 1: penyimpanan dan bantuan
package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var students []Student
var nextID = 1

func findStudentIndex(id int) int {
	for i := range students {
		if students[i].ID == id {
			return i
		}
	}
	return -1
}

func findStudentIndexByNIM(nim string) int {
	for i := range students {
		if students[i].NIM == nim {
			return i
		}
	}
	return -1
}

func cocokPencarian(s Student, kata string) bool {
	return strings.Contains(strings.ToLower(s.Nama), strings.ToLower(kata))
}

func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

func listStudents(c *fiber.Ctx) error {
	q := parseListQuery(c)

	hasil := []Student{}
	for _, s := range students {
		if q.IsActive != nil && s.IsActive != *q.IsActive {
			continue
		}
		if q.Search != "" && !cocokPencarian(s, q.Search) {
			continue
		}
		hasil = append(hasil, s)
	}

	sort.SliceStable(hasil, func(i, j int) bool {
		var lebihKecil bool
		switch q.Sort {
		case "name":
			lebihKecil = hasil[i].Nama < hasil[j].Nama
		case "grade":
			lebihKecil = hasil[i].Nilai < hasil[j].Nilai
		default:
			lebihKecil = hasil[i].ID < hasil[j].ID
		}
		if q.Order == "desc" {
			return !lebihKecil
		}
		return lebihKecil
	})

	total := len(hasil)
	totalPages := (total + q.Limit - 1) / q.Limit
	mulai := (q.Page - 1) * q.Limit
	if mulai > total {
		mulai = total
	}
	akhir := mulai + q.Limit
	if akhir > total {
		akhir = total
	}

	return okList(c, "daftar mahasiswa berhasil diambil", hasil[mulai:akhir], &Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

func getStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}
	return ok(c, "mahasiswa ditemukan", students[i])
}

func createStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Nama = strings.TrimSpace(req.Nama)

	errs := map[string]string{}
	if req.NIM == "" {
		errs["nim"] = "wajib diisi"
	}
	if req.Nama == "" {
		errs["name"] = "wajib diisi"
	}
	if req.Nilai < 0 || req.Nilai > 100 {
		errs["grade"] = "harus di antara 0 dan 100"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	if req.NIM != "" && findStudentIndexByNIM(req.NIM) != -1 {
		return fail(c, fiber.StatusConflict, "nim sudah dipakai")
	}

	baru := Student{
		ID:       nextID,
		NIM:      req.NIM,
		Nama:     req.Nama,
		Nilai:    req.Nilai,
		IsActive: true, 
	}
	students = append(students, baru)
	nextID++

	return created(c, "mahasiswa berhasil dibuat", baru,
		"/api/v1/students/"+strconv.Itoa(baru.ID))
}

func replaceStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Nama = strings.TrimSpace(req.Nama)

	errs := map[string]string{}
	if req.NIM == "" {
		errs["nim"] = "wajib diisi pada PUT"
	}
	if req.Nama == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if req.Nilai < 0 || req.Nilai > 100 {
		errs["grade"] = "wajib diisi dan berada di antara 0-100 pada PUT"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	if req.NIM != students[i].NIM {
		if j := findStudentIndexByNIM(req.NIM); j != -1 {
			return fail(c, fiber.StatusConflict, "nim sudah dipakai")
		}
	}

	students[i].NIM = req.NIM
	students[i].Nama = req.Nama
	students[i].Nilai = req.Nilai
	students[i].IsActive = req.IsActive

	return ok(c, "mahasiswa berhasil diganti seluruhnya", students[i])
}

func patchStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	if req.NIM == nil && req.Nama == nil && req.Nilai == nil && req.IsActive == nil {
		return fail(c, fiber.StatusBadRequest, "tidak ada field yang diubah")
	}

	if req.NIM != nil {
		nim := strings.TrimSpace(*req.NIM)
		if nim == "" {
			return failValidation(c, map[string]string{"nim": "tidak boleh kosong"})
		}
		if nim != students[i].NIM {
			if j := findStudentIndexByNIM(nim); j != -1 {
				return fail(c, fiber.StatusConflict, "nim sudah dipakai")
			}
		}
		students[i].NIM = nim
	}
	if req.Nama != nil {
		if strings.TrimSpace(*req.Nama) == "" {
			return failValidation(c, map[string]string{"name": "tidak boleh kosong"})
		}
		students[i].Nama = *req.Nama
	}
	if req.Nilai != nil {
		if *req.Nilai < 0 || *req.Nilai > 100 {
			return failValidation(c, map[string]string{"grade": "harus di antara 0 dan 100"})
		}
		students[i].Nilai = *req.Nilai
	}
	if req.IsActive != nil {
		students[i].IsActive = *req.IsActive
	}

	return ok(c, "mahasiswa berhasil diperbarui sebagian", students[i])
}

func deleteStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}
	students = append(students[:i], students[i+1:]...)
	return noContent(c)
}