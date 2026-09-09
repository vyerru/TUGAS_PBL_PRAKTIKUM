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