package main

import "fmt"

type Student struct {
	ID       int
	Name     string
	Grade    float64
	IsActive bool
}

func (s Student) GetInfo() string {
	status := "Aktif"
	if !s.IsActive {
		status = "Tidak Aktif"
	}
	return fmt.Sprintf("ID: %d | Nama: %s | Grade: %.2f | Status: %s", s.ID, s.Name, s.Grade, status)
}

func (s *Student) UpdateGrade(grade float64) {
	s.Grade = grade
}

func (s *Student) Activate() {
	s.IsActive = true
}

func (s *Student) Deactivate() {
	s.IsActive = false
}

func main() {
	mhs := Student{ID: 1, Name: "Rakha", Grade: 3.5, IsActive: true}

	fmt.Println("=== DATA AWAL ===")
	fmt.Println(mhs.GetInfo())

	mhs.UpdateGrade(3.8)
	fmt.Println("\n=== SETELAH UPDATE GRADE ===")
	fmt.Println(mhs.GetInfo())

	mhs.Deactivate()
	fmt.Println("\n=== SETELAH DEAKTIF ===")
	fmt.Println(mhs.GetInfo())

	mhs.Activate()
	fmt.Println("\n=== SETELAH AKTIF LAGI ===")
	fmt.Println(mhs.GetInfo())
}