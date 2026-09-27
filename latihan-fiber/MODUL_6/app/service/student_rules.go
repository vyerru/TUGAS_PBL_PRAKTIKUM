package service

import (
	"strings"

	"MODUL_5/app/model"
)

// File ini berisi business rules MURNI: tidak menyentuh fiber.Ctx,
// tidak menyentuh database, dan tidak tahu apa pun tentang HTTP.

func ValidateCreate(req model.CreateStudentRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi"
	}
	if strings.TrimSpace(req.NIM) == "" {
		errs["nim"] = "wajib diisi"
	}
	if strings.TrimSpace(req.Grade) == "" {
		errs["grade"] = "wajib diisi"
	}
	return errs
}

func ValidateReplace(req model.ReplaceStudentRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if strings.TrimSpace(req.Grade) == "" {
		errs["grade"] = "wajib diisi pada PUT"
	}
	return errs
}

func ApplyPatch(
	current model.Student, req model.PatchStudentRequest,
) (model.Student, map[string]string) {
	errs := map[string]string{}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			errs["name"] = "tidak boleh kosong"
		} else {
			current.Name = *req.Name
		}
	}
	if req.Grade != nil {
		if strings.TrimSpace(*req.Grade) == "" {
			errs["grade"] = "tidak boleh kosong"
		} else {
			current.Grade = *req.Grade
		}
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}

	return current, errs
}

func IsEmptyPatch(req model.PatchStudentRequest) bool {
	return req.Name == nil && req.Grade == nil && req.IsActive == nil
}

func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}