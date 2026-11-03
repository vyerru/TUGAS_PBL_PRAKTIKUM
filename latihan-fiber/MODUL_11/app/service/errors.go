package service

import (
	"errors"

	"MODUL_11/app/repository"
	"MODUL_11/helper"
)

func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("username sudah dipakai")
	default:
		return helper.Internal(err)
	}
}