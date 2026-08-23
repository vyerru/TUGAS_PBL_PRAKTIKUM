package main

import "fmt"

func main() {
	var nama string = "Rakha"
	var NIM int = 21
	var IPK float64 = 3.5
	var statusAktif bool = true
	var nilai []int = []int{100, 100, 100, 100, 100}

	fmt.Printf("Nama: %s \n NIM: %d \n IPK: %.2f \n Status Aktif: %t \n Nilai: %v \n", nama, NIM, IPK, statusAktif, nilai)

	mahasiswa := map[string][]int{
		"Rakha":  {100, 95, 88, 92, 90},
		"Javier":   {85, 78, 90, 88, 92},
		"Abhista":    {90, 92, 95, 85, 88},
	}

	mahasiswa["Muhammad"] = []int{80, 85, 90, 75, 88}

	for nama, nilai := range mahasiswa {
		fmt.Printf("Nama: %s | Nilai: %v\n", nama, nilai)
	}

	cariNama := "Rakha"
	nilai, ada := mahasiswa[cariNama] 
	if ada {
		fmt.Printf("Nama: %s | Nilai: %v\n", cariNama, nilai)
	} else {
		fmt.Printf("\n nama %s tidak ketemu\n", cariNama)
	}

	delete(mahasiswa, "Javier")
	for nama, nilai := range mahasiswa {
		fmt.Printf("Nama: %s | Nilai: %v\n", nama, nilai)
	}

	for nama, nilai := range mahasiswa {
		rataRata := 0.0
		for _, n := range nilai {
			rataRata += float64(n)	
		}
		rataRata /= float64(len(nilai))
		fmt.Printf("Nama: %s | Jumlah Mata Kuliah: %d | Rata-rata: %.2f\n", nama, len(nilai), rataRata)
	}
}