package service

import (
	"testing"
)

func TestCheckPasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantMsg  string
	}{
		{
			name:     "Password Kurang dari 8 Karakter",
			password: "short1",
			wantMsg:  "minimal 8 karakter",
		},
		{
			name:     "Password Tanpa Angka",
			password: "hanyahuruf",
			wantMsg:  "harus memuat huruf dan angka",
		},
		{
			name:     "Password Terlalu Umum (Lemah)",
			password: "password123",
			wantMsg:  "password terlalu umum",
		},
		{
			name:     "Password Kuat dan Valid",
			password: "KuatSekali99",
			wantMsg:  "", // Tidak mengembalikan error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg := checkPasswordStrength(tt.password)
			if gotMsg != tt.wantMsg {
				t.Errorf("checkPasswordStrength() = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}