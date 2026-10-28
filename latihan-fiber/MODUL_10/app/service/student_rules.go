package service

import (
	"strings"

	"MODUL_10/app/model"
)

// ApplyPatchStudent menggabungkan field patch yang tidak bernilai nil
// ke current student. Pemeriksaan bentuk sudah selesai dikerjakan tag
// sebelum fungsi ini dipanggil.
func ApplyPatchStudent(current model.Student, req model.PatchStudentRequest) model.Student {
	if req.Name != nil {
		current.Name = strings.TrimSpace(*req.Name)
	}
	if req.Grade != nil {
		current.Grade = strings.TrimSpace(*req.Grade)
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current
}

// IsEmptyStudentPatch memeriksa apakah body PATCH tidak memuat field apa pun.
func IsEmptyStudentPatch(req model.PatchStudentRequest) bool {
	return req.Name == nil && req.Grade == nil && req.IsActive == nil
}