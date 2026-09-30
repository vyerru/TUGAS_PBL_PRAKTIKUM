package service

import "strings"

// Helper ini dipakai bersama oleh seluruh service (user, student, auth).

// isValidEmail melakukan pengecekan sederhana format email.
func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	dot := strings.LastIndex(email, ".")
	return at > 0 && dot > at+1 && dot < len(email)-1
}

// CountTotalPages menghitung jumlah halaman dari total data dan limit.
func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
