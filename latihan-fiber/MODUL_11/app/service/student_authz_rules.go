package service

import (
	"MODUL_11/app/model"
	"MODUL_11/helper"
)

func CanAccessStudent(
	current model.AuthUser,
	ownerID int,
	perms *helper.PermissionSet,
	anyPermission string,
) bool {
	if current.UserID == ownerID {
		return true
	}
	return perms.Can(current.Role, anyPermission)
}