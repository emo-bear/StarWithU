package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// Event 是一条恋爱记录，同时携带它在星云里的位置（单位化坐标，前端乘以半径）。
type Event struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Date        string   `json:"date"` // YYYY-MM-DD
	Description string   `json:"description"`
	Images      []string `json:"images"` // 相对 images/ 的文件名
	X           float64  `json:"x"`
	Y           float64  `json:"y"`
	Z           float64  `json:"z"`
	CreatedAt   string   `json:"createdAt"`
}

// Store 是极简的 SQLite 封装，单文件、无网络、无后台服务。
type Store struct {
	mu  sync.Mutex
	db  *sql.DB
	dir string
}

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	title       TEXT    NOT NULL,
	date        TEXT    NOT NULL,
	description TEXT    NOT NULL DEFAULT '',
	images      TEXT    NOT NULL DEFAULT '[]',
	x           REAL    NOT NULL DEFAULT 0,
	y           REAL    NOT NULL DEFAULT 0,
	z           REAL    NOT NULL DEFAULT 0,
	created_at  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_events_date ON events(date);
`

// Open 打开（或创建）指定目录下的数据库。
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// 兼容早期版本的库文件名
	old := filepath.Join(dir, "lulu.db")
	dbPath := filepath.ToSlash(filepath.Join(dir, "star.db"))
	if _, err := os.Stat(filepath.Join(dir, "star.db")); os.IsNotExist(err) {
		if _, e := os.Stat(old); e == nil {
			_ = os.Rename(old, filepath.Join(dir, "star.db"))
			_ = os.Remove(old + "-wal")
			_ = os.Remove(old + "-shm")
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, dir: dir}
	if err := s.relayout(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// relayout 按 id 重新计算每条记录的星云位置。位置只由 id 决定，所以结果稳定。
func (s *Store) relayout() error {
	rows, err := s.db.Query(`SELECT id FROM events`)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, 64)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`UPDATE events SET x = ?, y = ?, z = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		x, y, z := nebulaPos(id)
		if _, err := stmt.Exec(x, y, z, id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// nebulaPos 给出第 id 条记录在星系盘上的位置（单位化，前端再乘半径）。
// 与前端一致的对数螺旋：phi = arm*PI + k*ln(r/r0)，两条旋臂。
func nebulaPos(id int64) (float64, float64, float64) {
	const (
		spiralK  = 2.9
		spiralR0 = 0.055
	)
	rnd := rand.New(rand.NewSource(id*7919 + 104729))

	// 避开最亮的核心，让事件落在盘面中外部
	r := 0.2 + 0.72*math.Pow(rnd.Float64(), 0.68)

	// 七成落在旋臂上，其余弥散在盘面里
	var phi float64
	if rnd.Float64() < 0.72 {
		arm := float64(rnd.Intn(2)) * math.Pi
		phi = arm + spiralK*math.Log(r/spiralR0) + rnd.NormFloat64()*0.09
	} else {
		phi = rnd.Float64() * 2 * math.Pi
	}

	// 盘面很薄，越靠外越薄
	y := rnd.NormFloat64() * 0.014 * (1 - 0.45*r)
	x := r * math.Cos(phi)
	z := r * math.Sin(phi)
	return x, y, z
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Dir() string { return s.dir }

func (s *Store) List() ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT id, title, date, description, images, x, y, z, created_at
		FROM events ORDER BY date ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Event, 0, 32)
	for rows.Next() {
		var e Event
		var images string
		if err := rows.Scan(&e.ID, &e.Title, &e.Date, &e.Description, &images, &e.X, &e.Y, &e.Z, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Images = decodeImages(images)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Get(id int64) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var e Event
	var images string
	err := s.db.QueryRow(`SELECT id, title, date, description, images, x, y, z, created_at
		FROM events WHERE id = ?`, id).
		Scan(&e.ID, &e.Title, &e.Date, &e.Description, &images, &e.X, &e.Y, &e.Z, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	e.Images = decodeImages(images)
	return e, nil
}

// Save 创建（ID==0）或更新一条记录。
func (s *Store) Save(e *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.Images == nil {
		e.Images = []string{}
	}
	images, _ := json.Marshal(e.Images)

	if e.ID == 0 {
		res, err := s.db.Exec(`INSERT INTO events (title, date, description, images, x, y, z, created_at)
			VALUES (?, ?, ?, ?, 0, 0, 0, datetime('now','localtime'))`,
			e.Title, e.Date, e.Description, string(images))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		e.ID = id
		e.X, e.Y, e.Z = nebulaPos(id)
		_, err = s.db.Exec(`UPDATE events SET x = ?, y = ?, z = ? WHERE id = ?`, e.X, e.Y, e.Z, id)
		return err
	}

	_, err := s.db.Exec(`UPDATE events SET title = ?, date = ?, description = ?, images = ? WHERE id = ?`,
		e.Title, e.Date, e.Description, string(images), e.ID)
	return err
}

func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`DELETE FROM events WHERE id = ?`, id); err != nil {
		return err
	}
	return nil
}

func (s *Store) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n)
	return n, err
}

func decodeImages(raw string) []string {
	out := []string{}
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	if out == nil {
		out = []string{}
	}
	return out
}
