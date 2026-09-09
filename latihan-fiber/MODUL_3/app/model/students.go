package model

import "time"

type Student struct {
	ID        int       `json:"id"`
	NIM       string    `json:"nim"`
	Name      string    `json:"name"`
	Grade     string    `json:"grade"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateStudentRequest struct {
	NIM   string `json:"nim"`
	Name  string `json:"name"`
	Grade string `json:"grade"`
}

type ReplaceStudentRequest struct {
	Name     string `json:"name"`
	Grade    string `json:"grade"`
	IsActive bool   `json:"is_active"`
}

type PatchStudentRequest struct {
	Name     *string `json:"name"`
	Grade    *string `json:"grade"`
	IsActive *bool   `json:"is_active"`
}