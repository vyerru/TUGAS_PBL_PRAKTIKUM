package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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
	close(start) // buka gerbang
	wg.Wait()
	close(results)

	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

type testUser struct {
	ID       int
	Username string
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

func (j *jwtManager) GenerateAccessToken(userID int, username, role string) (string, error) {
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

func main() {
	mode := flag.String("mode", "crowd", "Mode uji beban: crowd, credits, swap")
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("gagal konek database: %v", err)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(ctx, `
		SELECT id, username FROM users
		WHERE username LIKE 'mhs%' AND username <> 'mhs_test'
		ORDER BY username`)
	if err != nil {
		log.Fatalf("gagal mengambil user uji: %v", err)
	}
	defer rows.Close()

	users := []testUser{}
	for rows.Next() {
		var u testUser
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			log.Fatalf("gagal membaca user: %v", err)
		}
		users = append(users, u)
	}
	if len(users) < 60 {
		log.Fatalf("butuh 60 akun uji, hanya ditemukan %d (jalankan migrations/006_seed_loadtest.sql)", len(users))
	}
	users = users[:60]

	jwtMgr := newJWTManager(jwtSecret, jwtIssuer, 15*time.Minute)
	tokens := make([]string, len(users))
	for i, u := range users {
		token, err := jwtMgr.GenerateAccessToken(u.ID, u.Username, "user")
		if err != nil {
			log.Fatalf("gagal buat token %s: %v", u.Username, err)
		}
		tokens[i] = token
	}

	courseIDs := courseIDMap(ctx, conn)

	client := &http.Client{Timeout: 30 * time.Second}

	var results map[string]int
	start := time.Now()

	switch *mode {
	case "crowd":
		// 60 mahasiswa berebut kelas IF399 (kuota 5)
		target := courseIDs["IF399"]
		results = fire(60, func(i int) string {
			return enroll(client, baseURL, tokens[i], target)
		})
	case "credits":
		// 10 mahasiswa, masing-masing mendaftar ke 12 kelas sekaligus (36 SKS > 24)
		wanted := []string{"IF101", "IF102", "IF301", "IF302", "IF303", "IF304",
			"IF305", "IF306", "IF307", "IF308", "IF309", "IF310"}
		ids := make([]int, 0, len(wanted))
		for _, code := range wanted {
			ids = append(ids, courseIDs[code])
		}
		results = fire(10, func(i int) string {
			return enrollMultiple(client, baseURL, tokens[i], ids)
		})
	case "swap":
		// 20 pasang mahasiswa pindah kelas berlawanan arah (IF101 <-> IF102).
		// Setiap mahasiswa harus SUDAH terdaftar di kelas asalnya sebelum
		// bisa pindah, jadi kita daftarkan dulu satu per satu.
		from, to := courseIDs["IF101"], courseIDs["IF102"]
		pairs := 20
		for i := 0; i < pairs*2; i++ {
			target := from
			if i%2 == 1 {
				target = to
			}
			if status := enroll(client, baseURL, tokens[i], target); status != "201" {
				log.Fatalf("setup gagal: mahasiswa %d daftar kelas %d -> %s", i, target, status)
			}
		}
		results = fire(pairs*2, func(i int) string {
			if i%2 == 0 {
				return swap(client, baseURL, tokens[i], from, to)
			}
			return swap(client, baseURL, tokens[i], to, from)
		})
	default:
		log.Fatalf("mode tidak dikenal: %s", *mode)
	}

	printResults(*mode, results, time.Since(start))
}

func courseIDMap(ctx context.Context, conn *pgx.Conn) map[string]int {
	rows, err := conn.Query(ctx, "SELECT code, id FROM courses")
	if err != nil {
		log.Fatalf("gagal mengambil kelas: %v", err)
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var code string
		var id int
		if err := rows.Scan(&code, &id); err != nil {
			log.Fatalf("gagal membaca kelas: %v", err)
		}
		result[code] = id
	}
	return result
}

func enroll(client *http.Client, baseURL, token string, courseID int) string {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/courses/%d/enroll", baseURL, courseID),
		bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "ERR"
	}
	defer resp.Body.Close()
	return fmt.Sprintf("%d", resp.StatusCode)
}

func enrollMultiple(client *http.Client, baseURL, token string, courseIDs []int) string {
	success := 0
	for _, id := range courseIDs {
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/courses/%d/enroll", baseURL, id),
			bytes.NewReader([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
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
	req, _ := http.NewRequest("POST", baseURL+"/enrollments/swap", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "ERR"
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
