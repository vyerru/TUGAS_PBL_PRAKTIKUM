package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fire menjalankan n pekerjaan secara serentak dan menghitung hasilnya.
//
// Setiap goroutine menunggu di depan "gerbang" start yang sama. Gerbang
// baru dibuka setelah SELURUH goroutine siap, sehingga request benar-benar
// berangkat hampir bersamaan -- bukan bergiliran sesuai urutan dibuatnya.
func fire(n int, job func(i int) string) map[string]int {
	var wg sync.WaitGroup
	start := make(chan struct{})

	results := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, out chan<- string) {
			defer wg.Done()
			<-start
			out <- job(i)
		}(i, results)
	}
	close(start) // buka gerbang
	wg.Wait()
	// tunggu SEMUA goroutine selesai
	close(results)
	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

type jwtManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func newJWTManager(secret, issuer string, accessTTL time.Duration) *jwtManager {
	return &jwtManager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}
}

func (j *jwtManager) GenerateAccessToken(userID int, username, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"role":     role,
		"iss":      j.issuer,
		"exp":      time.Now().Add(j.accessTTL).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func main() {
	mode := flag.String("mode", "crowd", "Mode uji beban: crowd, credits, swap")
	flag.Parse()

	baseURL := "http://localhost:3000/api/v1"
	jwtSecret := "626e7d1454a737bf8fbc16cd6c093245ed824269ff628144b26c36e798d3364b"
	jwtIssuer := "praktikum-backend"
	jwtAccessTTL := 15 * time.Minute

	jwtMgr := newJWTManager(jwtSecret, jwtIssuer, jwtAccessTTL)

	// Buat token untuk 60 mahasiswa uji (mhs001..mhs060)
	tokens := make([]string, 60)
	for i := 1; i <= 60; i++ {
		username := fmt.Sprintf("mhs%03d", i)
		token, err := jwtMgr.GenerateAccessToken(i, username, "user")
		if err != nil {
			log.Fatalf("gagal buat token %s: %v", username, err)
		}
		tokens[i-1] = token
	}

	client := &http.Client{Timeout: 10 * time.Second}

	var results map[string]int
	start := time.Now()

	switch *mode {
	case "crowd":
		// 60 mahasiswa berebut kelas IF399 (kuota 5)
		// IF399 id biasanya 13 (lihat migration 005)
		results = fire(60, func(i int) string {
			return enroll(client, baseURL, tokens[i], 13) // IF399
		})
	case "credits":
		// 10 mahasiswa, masing-masing mendaftar ke 12 kelas sekaligus (batas 24 SKS)
		// Setiap mahasiswa ambil 12 kelas x 3 SKS = 36 SKS > 24
		results = fire(10, func(i int) string {
			return enrollMultiple(client, baseURL, tokens[i])
		})
	case "swap":
		// 20 pasang mahasiswa pindah kelas berlawanan arah (IF101 <-> IF102)
		// IF101 id=1, IF102 id=2
		results = fire(20, func(i int) string {
			// Genap: IF101 -> IF102, Ganjil: IF102 -> IF101
			if i%2 == 0 {
				return swap(client, baseURL, tokens[i], 1, 2)
			}
			return swap(client, baseURL, tokens[i], 2, 1)
		})
	default:
		log.Fatalf("mode tidak dikenal: %s", *mode)
	}

	elapsed := time.Since(start)
	printResults(*mode, results, elapsed)
}

func enroll(client *http.Client, baseURL, token string, courseID int) string {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/courses/%d/enroll", baseURL, courseID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "ERR:" + err.Error()
	}
	defer resp.Body.Close()
	return fmt.Sprintf("%d", resp.StatusCode)
}

func enrollMultiple(client *http.Client, baseURL, token string) string {
	// Daftar 12 kelas (IF101..IF310, skip IF399)
	courseIDs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	success := 0
	for _, id := range courseIDs {
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/courses/%d/enroll", baseURL, id), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 201 {
			success++
		}
	}
	return fmt.Sprintf("OK:%d", success)
}

func swap(client *http.Client, baseURL, token string, fromID, toID int) string {
	payload := map[string]int{"from_course_id": fromID, "to_course_id": toID}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/enrollments/swap", baseURL), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "ERR:" + err.Error()
	}
	defer resp.Body.Close()
	return fmt.Sprintf("%d", resp.StatusCode)
}

func printResults(mode string, results map[string]int, elapsed time.Duration) {
	fmt.Printf("\n=== %s ===\n", mode)
	fmt.Printf("Waktu total: %v\n", elapsed)
	fmt.Println("Hasil request:")
	total := 0
	for code, count := range results {
		fmt.Printf("  %s: %d\n", code, count)
		total += count
	}
	fmt.Printf("Total: %d\n", total)
}