package service

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"MODUL_10/app/model"
	"MODUL_10/app/repository"
	"MODUL_10/helper"
)

type SupervisorService struct {
	supervisors repository.SupervisorRepository
	tx          *repository.TxManager
}

func NewSupervisorService(supervisors repository.SupervisorRepository, tx *repository.TxManager) *SupervisorService {
	return &SupervisorService{supervisors: supervisors, tx: tx}
}

// ---------- GET /supervisors ----------
func (s *SupervisorService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	list, err := s.supervisors.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "daftar pembimbing berhasil diambil", list)
}

// ---------- GET /supervisors/:id ----------
func (s *SupervisorService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	sup, err := s.supervisors.FindByID(ctx, id)
	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Success(c, fiber.StatusOK, "pembimbing ditemukan", sup)
}

// ---------- POST /supervisors/:id/assign ----------
// Penetapan pembimbing tugas akhir:
// Menggunakan UPDATE bersyarat untuk kuota dan kunci baris students
// untuk mencegah satu mahasiswa ditetapkan ke dua pembimbing secara serentak.
func (s *SupervisorService) Assign(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	supervisorID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	var assigned model.Supervisor
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.supervisors.WithTx(tx)

		// 1. Kunci baris mahasiswa (Pessimistic Locking)
		// Mencegah mahasiswa yang sama ditetapkan ke dua pembimbing secara serentak
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		// 2. Periksa apakah mahasiswa sudah memiliki pembimbing
		existing, err := repo.FindByStudentID(ctx, req.StudentID)
		if err == nil && existing.ID != 0 {
			return repository.ErrAlreadySupervised
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		// 3. Ambil satu kuota pembimbing secara atomik (UPDATE bersyarat)
		assigned, err = repo.IncrementSupervised(ctx, supervisorID)
		if err != nil {
			return err
		}

		// 4. Catat penetapan pembimbing
		return repo.InsertSupervision(ctx, req.StudentID, supervisorID)
	})
	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Created(c, "berhasil menetapkan dosen pembimbing", assigned,
		"/api/v1/supervisors/"+strconv.Itoa(supervisorID))
}

// ---------- POST /supervisions/switch ----------
// Pergantian dosen pembimbing tugas akhir:
// Wajib atomik — bila pembimbing baru penuh, mahasiswa tetap dengan pembimbing lama —
// dan wajib bebas deadlock (penguncian seragam dengan urutan id menaik).
func (s *SupervisorService) Switch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.SwitchSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if req.FromSupervisorID == req.ToSupervisorID {
		return helper.Validation(map[string]string{
			"to_supervisor_id": "tidak boleh sama dengan from_supervisor_id",
		})
	}

	var joined model.Supervisor
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.supervisors.WithTx(tx)

		// 1. Kunci baris mahasiswa
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		// 2. Kunci KEDUA pembimbing secara seragam dalam urutan ID menaik (Bebas Deadlock)
		if err := repo.LockSupervisors(ctx, req.FromSupervisorID, req.ToSupervisorID); err != nil {
			return err
		}

		// 3. Verifikasi bahwa mahasiswa saat ini memang dibimbing oleh FromSupervisorID
		existing, err := repo.FindByStudentID(ctx, req.StudentID)
		if err != nil {
			return repository.ErrNotSupervised
		}
		if existing.SupervisorID != req.FromSupervisorID {
			return repository.ErrNotSupervised
		}

		// 4. Hapus bimbingan lama
		if err := repo.DeleteSupervision(ctx, req.StudentID, req.FromSupervisorID); err != nil {
			return err
		}

		// 5. Kurangi bimbingan pembimbing lama
		if _, err := repo.DecrementSupervised(ctx, req.FromSupervisorID); err != nil {
			return err
		}

		// 6. Tambah bimbingan pembimbing baru secara atomik (UPDATE bersyarat)
		// Bila pembimbing baru penuh, error membatalkan langkah 4 dan 5 (Rollback),
		// sehingga mahasiswa tetap dengan pembimbing lama
		joined, err = repo.IncrementSupervised(ctx, req.ToSupervisorID)
		if err != nil {
			return err
		}

		// 7. Catat bimbingan baru
		return repo.InsertSupervision(ctx, req.StudentID, req.ToSupervisorID)
	})
	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Success(c, fiber.StatusOK, "berhasil mengganti dosen pembimbing", joined)
}

// ---------- PATCH /supervisors/:id ----------
// Mengubah kuota pembimbing dengan Optimistic Locking
func (s *SupervisorService) UpdateQuota(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.UpdateSupervisorQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	updated, err := s.supervisors.UpdateQuota(ctx, id, req.Quota, req.Version)
	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Success(c, fiber.StatusOK, "kuota pembimbing berhasil diubah", updated)
}

func translateSupervisorError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("data tidak ditemukan")
	case errors.Is(err, repository.ErrSupervisorFull):
		return helper.NewError(fiber.StatusConflict, "SUPERVISOR_FULL", "kuota pembimbing sudah penuh")
	case errors.Is(err, repository.ErrAlreadySupervised):
		return helper.NewError(fiber.StatusConflict, "ALREADY_SUPERVISED", "mahasiswa sudah memiliki pembimbing")
	case errors.Is(err, repository.ErrNotSupervised):
		return helper.NewError(fiber.StatusConflict, "NOT_SUPERVISED", "mahasiswa tidak dibimbing oleh dosen tersebut")
	case errors.Is(err, repository.ErrSupervisorVersionConflict):
		return helper.NewError(fiber.StatusConflict, "VERSION_CONFLICT", "data pembimbing sudah diubah pihak lain, muat ulang lalu coba lagi")
	case errors.Is(err, repository.ErrQuotaBelowSupervised):
		return helper.NewError(fiber.StatusConflict, "QUOTA_BELOW_SUPERVISED", "kuota tidak boleh lebih kecil dari jumlah mahasiswa bimbingan saat ini")
	default:
		return helper.Internal(err)
	}
}
