package main

import "time"

type Student struct {
	ID       int `json:"id"`
	Nama     string `json:"name"`
	Nilai    float64 `json:"grade"`
	IsActive bool `json:"is_active"`
	NIM string `json:"nim"`
}

type CreateStudentRequest struct {
	Nama string `json:"name"`
	Nilai float64 `json:"grade"`
	IsActive bool `json:"is_active"`
	NIM string `json:"nim"`
}

type ReplaceStudentRequest struct {
	Nama string `json:"name"`
	Nilai float64 `json:"grade"`
	IsActive bool `json:"is_active"`
	NIM string `json:"nim"`
}

type PatchStudentRequest struct {
	Nama *string `json:"name,omitempty"`
	Nilai *float64 `json:"grade,omitempty"`
	IsActive *bool `json:"is_active,omitempty"`
	NIM *string `json:"nim,omitempty"`
}

type WebResponse struct {
	Succsess bool `json:"success"`
	Message string `json:"message"`
	Data any `json:"data,omitempty"`
	Meta *Meta `json:"meta,omitempty"`
	Errors any `json:"errors,omitempty"`
}

type Meta struct {
	Page int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type ListQuery struct {
	Page int
	Limit int
	Search string
	Sort string
	Order string
	IsActive *bool
}