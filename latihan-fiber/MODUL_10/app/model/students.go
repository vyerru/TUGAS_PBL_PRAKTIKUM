package model

import "time"

type Student struct {
	ID        int       `json:"id"`
	NIM       string    `json:"nim"`
	Name      string    `json:"name"`
	Grade     string    `json:"grade"`
	IsActive  bool      `json:"is_active"`
	OwnerID   int       `json:"owner_id"` 
	CreatedAt time.Time `json:"created_at"`
}

type CreateStudentRequest struct {
	NIM   string `json:"nim" validate:"required,validnim"`
	Name  string `json:"name" validate:"required,min=3,max=100"`
	Grade string `json:"grade" validate:"required,min=1,max=10"`
}

type ReplaceStudentRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=100"`
	Grade    string `json:"grade" validate:"required,min=1,max=10"`
	IsActive bool   `json:"is_active"`
}

type PatchStudentRequest struct {
	Name     *string `json:"name,omitempty" validate:"omitnil,min=3,max=100"`
	Grade    *string `json:"grade,omitempty" validate:"omitnil,min=1,max=10"`
	IsActive *bool   `json:"is_active,omitempty"`
}