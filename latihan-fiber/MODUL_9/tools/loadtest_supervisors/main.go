package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
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
	close(start) // buka gerbang: semua goroutine berangkat serentak
	wg.Wait()
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
	return &jwtManager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

type accessClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (j *jwtManager) generateToken(userID int, username, role string) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(userID),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// label mengubah response HTTP menjadi label ringkas untuk tally.
func label(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &parsed)
	if parsed.Code != "" && resp.StatusCode >= 400 {
		return fmt.Sprintf("%d %s", resp.StatusCode, parsed.Code)
	}
	return strconv.Itoa(resp.StatusCode)
}

func doAssign(client *http.Client, baseURL, token string, supervisorID, studentID int) string {
	payload := map[string]int{"student_id": studentID}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/supervisors/%d/assign", baseURL, supervisorID),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "ERR"
	}
	defer resp.Body.Close()
	return label(resp)
}

func doSwitch(client *http.Client, baseURL, token string, studentID, fromID, toID int) string {
	payload := map[string]int{
		"student_id":         studentID,
		"from_supervisor_id": fromID,
		"to_supervisor_id":   toID,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/supervisions/switch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "ERR"
	}
	defer resp.Body.Close()
	return label(resp)
}

func printResults(mode string, results map[string]int, elapsed time.Duration) {
	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("\n=== %s ===\n", mode)
	total := 0
	for _, k := range keys {
		fmt.Printf("  %-35s %d\n", k, results[k])
		total += results[k]
	}
	fmt.Printf("  %-35s %d\n", "TOTAL", total)
	fmt.Printf("waktu total : %v\n", elapsed)
}

func main() {
	mode := flag.String("mode", "assign", "Mode uji beban: assign, switch, assign_broken")
	flag.Parse()

	baseURL := env("LOADTEST_BASE_URL", "http://localhost:3000/api/v1")
	jwtSecret := env("JWT_SECRET", "626e7d1454a737bf8fbc16cd6c093245ed824269ff628144b26c36e798d3364b")
	jwtIssuer := env("JWT_ISSUER", "praktikum-backend")

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		env("DB_USER", "postgres"),
		env("DB_PASSWORD", "123"),
		env("DB_HOST", "localhost"),
		env("DB_PORT", "5432"),
		env("DB_NAME", "praktikum_backend"),
		env("DB_SSLMODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("gagal konek database: %v", err)
	}
	defer conn.Close(context.Background())

	// Ambil admin user untuk token (memiliki permission supervision:manage)
	var adminID int
	var adminUsername string
	err = conn.QueryRow(ctx, `
		SELECT u.id, u.username
		FROM users u
		JOIN role_permissions rp ON rp.role_name = u.role
		WHERE rp.permission_name = 'supervision:manage'
		ORDER BY u.id LIMIT 1`).Scan(&adminID, &adminUsername)
	if err != nil {
		log.Fatalf("gagal mengambil user dengan permission supervision:manage: %v", err)
	}

	jwtMgr := newJWTManager(jwtSecret, jwtIssuer, 15*time.Minute)
	adminToken, err := jwtMgr.generateToken(adminID, adminUsername, "admin")
	if err != nil {
		log.Fatalf("gagal buat token admin: %v", err)
	}

	// Ambil ID 60 mahasiswa uji (nim LIKE '434241021%')
	rows, err := conn.Query(ctx, `
		SELECT id FROM students
		WHERE nim LIKE '434241021%'
		ORDER BY id LIMIT 60`)
	if err != nil {
		log.Fatalf("gagal mengambil mahasiswa uji: %v", err)
	}
	studentIDs := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			log.Fatalf("gagal membaca id mahasiswa: %v", err)
		}
		studentIDs = append(studentIDs, id)
	}
	rows.Close()
	if len(studentIDs) < 30 {
		log.Fatalf("butuh minimal 30 mahasiswa uji, hanya ditemukan %d (jalankan migrations/007_supervisors.sql)", len(studentIDs))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var testResults map[string]int
	start := time.Now()

	switch *mode {
	case "assign":
		// 30 mahasiswa berebut pembimbing berkuota 3 secara serentak.
		// Spesifikasi D.3: harus menghasilkan TEPAT 3 keberhasilan (201).
		fmt.Println("30 mahasiswa berebut pembimbing #1 (kuota 3) secara serentak...")
		testResults = fire(30, func(i int) string {
			return doAssign(client, baseURL, adminToken, 1, studentIDs[i])
		})

	case "assign_broken":
		// Skenario pembanding (D.3): kode sementara tanpa penguncian kuota (versi naif).
		// Endpoint ini harus MAMPU mendeteksi kelebihan bimbingan.
		// PENTING: jalankan reset_supervisors.sql sebelum uji ini.
		fmt.Println("[PEMBANDING] 30 mahasiswa berebut pembimbing #1 secara serentak (tanpa guard)...")
		testResults = fire(30, func(i int) string {
			return doAssign(client, baseURL, adminToken, 1, studentIDs[i])
		})

	case "switch":
		// 20 pasang mahasiswa switch pembimbing berlawanan arah (1 <-> 2).
		// Setiap mahasiswa harus SUDAH terdaftar di pembimbing asalnya.
		pairs := 20
		if len(studentIDs) < pairs*2 {
			log.Fatalf("butuh %d mahasiswa uji untuk mode switch", pairs*2)
		}

		fmt.Println("Setup: mendaftarkan mahasiswa ke pembimbing asal satu per satu...")
		for i := 0; i < pairs*2; i++ {
			targetSup := 1
			if i%2 == 1 {
				targetSup = 2
			}
			status := doAssign(client, baseURL, adminToken, targetSup, studentIDs[i])
			if status != "201" {
				log.Fatalf("setup gagal: mahasiswa %d daftar pembimbing %d -> %s", studentIDs[i], targetSup, status)
			}
		}

		fmt.Printf("\n%d pasang mahasiswa switch pembimbing berlawanan arah secara serentak...\n", pairs)
		testResults = fire(pairs*2, func(i int) string {
			if i%2 == 0 {
				// pasangan genap: pindah dari pembimbing 1 ke 2
				return doSwitch(client, baseURL, adminToken, studentIDs[i], 1, 2)
			}
			// pasangan ganjil: pindah dari pembimbing 2 ke 1
			return doSwitch(client, baseURL, adminToken, studentIDs[i], 2, 1)
		})

	default:
		log.Fatalf("mode tidak dikenal: %s", *mode)
	}

	printResults(*mode, testResults, time.Since(start))
}
