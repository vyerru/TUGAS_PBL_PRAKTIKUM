package service

import (
	"errors"

	"MODUL_7/app/repository"
	"MODUL_7/helper"
)

func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		if entity == "student" {
			return helper.Conflict("NIM sudah dipakai")
		}
		return helper.Conflict("username sudah dipakai")
	default:
		return helper.Internal(err) 
	}
}