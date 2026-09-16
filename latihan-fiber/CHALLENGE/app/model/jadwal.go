package model

import (
	"time"

)

type Jadwal struct {
	ID_Jadwal        int       `json:"idjadwal"`
	Hari      time.Time    `json:"hari"`
	MataKuliah      string    `json:"matakuliah"`
}