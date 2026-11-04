package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"MODUL_11/app/model"
	"MODUL_11/config"
	"MODUL_11/database"
)

// Alat uji Bagian C.5: menaruh beberapa SALINAN task yang sama langsung ke
// Redis, meniru jaminan at-least-once (task yang sama datang berkali-kali).
//
// Penggunaan:
//
//	go run ./tools/redeliver -outbox <id> -copies 5
func main() {
	outboxID := flag.Int64("outbox", 0, "id baris outbox yang task-nya akan disalin")
	copies := flag.Int("copies", 1, "jumlah salinan task yang ditaruh ke Redis")
	flag.Parse()

	if *outboxID <= 0 || *copies <= 0 {
		fmt.Fprintln(os.Stderr, "penggunaan: go run ./tools/redeliver -outbox <id> -copies <n>")
		os.Exit(1)
	}

	config.LoadEnv()
	ctx := context.Background()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal terhubung ke database:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var eventType string
	var payload []byte
	var publishedAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT event_type, payload, published_at FROM outbox WHERE id = $1`,
		*outboxID).Scan(&eventType, &payload, &publishedAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal membaca outbox %d: %v\n", *outboxID, err)
		os.Exit(1)
	}

	// Payload task sama persis dengan yang dibuat relay, sehingga worker
	// mengenali salinan ini seperti task asli.
	job := model.NotificationJob{
		OutboxID: *outboxID, EventType: eventType, Payload: payload,
	}
	body, err := json.Marshal(job)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal menyusun payload task:", err)
		os.Exit(1)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()

	for i := 1; i <= *copies; i++ {
		task := asynq.NewTask(model.TaskSendNotification, body,
			// TaskID berbeda dari task milik relay ("outbox-<id>") agar
			// salinan ini benar-benar diterima sebagai task terpisah.
			asynq.TaskID(fmt.Sprintf("redeliver-%d-%d", *outboxID, i)),
			asynq.Queue(model.QueueNotifications),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
		)
		info, err := client.Enqueue(task)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gagal enqueue salinan %d: %v\n", i, err)
			os.Exit(1)
		}
		fmt.Printf("salinan %d: id=%s queue=%s\n", i, info.ID, info.Queue)
	}

	fmt.Printf("%d salinan task outbox %d (%s) ditaruh di queue %s (published_at=%v)\n",
		*copies, *outboxID, eventType, model.QueueNotifications, publishedAt)
}
