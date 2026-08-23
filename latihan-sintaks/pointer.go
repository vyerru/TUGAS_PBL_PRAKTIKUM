package main

import "fmt"

func swap(a, b *int) {
	*a, *b = *b, *a
}

func swapValue(a, b int) {
	a, b = b, a
}

func updateSlice(s *[]string, newItem string) {
	*s = append(*s, newItem)
}

func updateSliceValue(s []string, newItem string) {
	s = append(s, newItem)
}

func main() {
	// === SWAP PASS BY VALUE ===
	fmt.Println("=== PASS BY VALUE - SWAP ===")
	x1, x2 := 5, 10
	fmt.Println("Sebelum:", "x1 =", x1, ", x2 =", x2)
	swapValue(x1, x2)
	fmt.Println("Setelah: ", "x1 =", x1, ", x2 =", x2)

	// === SWAP PASS BY POINTER ===
	fmt.Println("\n=== PASS BY POINTER - SWAP ===")
	y1, y2 := 5, 10
	fmt.Println("Sebelum:", "y1 =", y1, ", y2 =", y2)
	swap(&y1, &y2)
	fmt.Println("Setelah: ", "y1 =", y1, ", y2 =", y2)

	// === UPDATE SLICE PASS BY VALUE ===
	fmt.Println("\n=== PASS BY VALUE - UPDATE SLICE ===")
	s1 := []string{"Rakha", "Javier"}
	fmt.Println("Sebelum:", s1)
	updateSliceValue(s1, "Abhista")
	fmt.Println("Setelah: ", s1)

	// === UPDATE SLICE PASS BY POINTER ===
	fmt.Println("\n=== PASS BY POINTER - UPDATE SLICE ===")
	s2 := []string{"Rakha", "Javier"}
	fmt.Println("Sebelum:", s2)
	updateSlice(&s2, "Abhista")
	fmt.Println("Setelah: ", s2)
}
