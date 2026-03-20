package db

import (
	"database/sql"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

type Server struct {
	ID              int64
	Domain          string
	Auth            string
	Harga           int64
	NamaServer      string
	Quota           int64
	IPLimit         int64
	BatasCreateAkun int64
	TotalCreateAkun int64
}

type PendingDeposit struct {
	UniqueCode     string
	UserID         int64
	Amount         int64
	OriginalAmount int64
	Timestamp      int64
	Status         string
	QRMessageID    int64
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER UNIQUE, saldo INTEGER DEFAULT 0);`,
		`CREATE TABLE IF NOT EXISTS pending_deposits (unique_code TEXT PRIMARY KEY, user_id INTEGER, amount INTEGER, original_amount INTEGER, timestamp INTEGER, status TEXT, qr_message_id INTEGER);`,
		`CREATE TABLE IF NOT EXISTS transactions (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, amount INTEGER, type TEXT, reference_id TEXT UNIQUE, timestamp INTEGER);`,
		`CREATE TABLE IF NOT EXISTS server (id INTEGER PRIMARY KEY AUTOINCREMENT, domain TEXT, auth TEXT, harga INTEGER, nama_server TEXT, quota INTEGER, iplimit INTEGER, batas_create_akun INTEGER, total_create_akun INTEGER DEFAULT 0);`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EnsureUser(userID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO users (user_id, saldo) VALUES (?, 0)`, userID)
	return err
}

func (s *Store) GetBalance(userID int64) (int64, error) {
	var saldo int64
	err := s.DB.QueryRow(`SELECT saldo FROM users WHERE user_id = ?`, userID).Scan(&saldo)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return saldo, err
}

func (s *Store) UpdateBalance(userID, amount int64) error {
	_, err := s.DB.Exec(`UPDATE users SET saldo = saldo + ? WHERE user_id = ?`, amount, userID)
	return err
}

func (s *Store) AddSaldoAdmin(userID, amount int64) error {
	if err := s.EnsureUser(userID); err != nil {
		return err
	}
	if err := s.UpdateBalance(userID, amount); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO transactions (user_id, amount, type, reference_id, timestamp) VALUES (?, ?, 'admin_addsaldo', ?, unixepoch())`, userID, amount, fmt.Sprintf("admin-%d-%d", userID, amount))
	return err
}

func (s *Store) AddTopup(userID, originalAmount, paidAmount int64, referenceID string) error {
	if err := s.EnsureUser(userID); err != nil {
		return err
	}
	res, err := s.DB.Exec(`INSERT OR IGNORE INTO transactions (user_id, amount, type, reference_id, timestamp) VALUES (?, ?, 'topup', ?, unixepoch())`, userID, originalAmount, referenceID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil
	}
	_, err = s.DB.Exec(`UPDATE users SET saldo = saldo + ? WHERE user_id = ?`, originalAmount, userID)
	return err
}

func (s *Store) DeductBalance(userID, amount int64, ref string) error {
	if err := s.EnsureUser(userID); err != nil {
		return err
	}
	var saldo int64
	if err := s.DB.QueryRow(`SELECT saldo FROM users WHERE user_id = ?`, userID).Scan(&saldo); err != nil {
		return err
	}
	if saldo < amount {
		return fmt.Errorf("saldo tidak cukup")
	}
	res, err := s.DB.Exec(`INSERT OR IGNORE INTO transactions (user_id, amount, type, reference_id, timestamp) VALUES (?, ?, 'purchase', ?, unixepoch())`, userID, -amount, ref)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil
	}
	_, err = s.DB.Exec(`UPDATE users SET saldo = saldo - ? WHERE user_id = ?`, amount, userID)
	return err
}

func (s *Store) CreatePendingDeposit(p PendingDeposit) error {
	_, err := s.DB.Exec(`INSERT INTO pending_deposits (unique_code, user_id, amount, original_amount, timestamp, status, qr_message_id) VALUES (?, ?, ?, ?, ?, ?, ?)`, p.UniqueCode, p.UserID, p.Amount, p.OriginalAmount, p.Timestamp, p.Status, p.QRMessageID)
	return err
}

func (s *Store) ListPendingDeposits() ([]PendingDeposit, error) {
	rows, err := s.DB.Query(`SELECT unique_code, user_id, amount, original_amount, timestamp, status, qr_message_id FROM pending_deposits WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingDeposit
	for rows.Next() {
		var p PendingDeposit
		if err := rows.Scan(&p.UniqueCode, &p.UserID, &p.Amount, &p.OriginalAmount, &p.Timestamp, &p.Status, &p.QRMessageID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeletePendingDeposit(uniqueCode string) error {
	_, err := s.DB.Exec(`DELETE FROM pending_deposits WHERE unique_code = ?`, uniqueCode)
	return err
}

func (s *Store) AddServer(srv Server) error {
	_, err := s.DB.Exec(`INSERT INTO server (domain, auth, harga, nama_server, quota, iplimit, batas_create_akun, total_create_akun) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, srv.Domain, srv.Auth, srv.Harga, srv.NamaServer, srv.Quota, srv.IPLimit, srv.BatasCreateAkun, srv.TotalCreateAkun)
	return err
}

func (s *Store) ListServers() ([]Server, error) {
	rows, err := s.DB.Query(`SELECT id, domain, auth, harga, nama_server, quota, iplimit, batas_create_akun, total_create_akun FROM server ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		var srv Server
		if err := rows.Scan(&srv.ID, &srv.Domain, &srv.Auth, &srv.Harga, &srv.NamaServer, &srv.Quota, &srv.IPLimit, &srv.BatasCreateAkun, &srv.TotalCreateAkun); err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

func (s *Store) GetServer(id int64) (*Server, error) {
	var srv Server
	err := s.DB.QueryRow(`SELECT id, domain, auth, harga, nama_server, quota, iplimit, batas_create_akun, total_create_akun FROM server WHERE id = ?`, id).Scan(&srv.ID, &srv.Domain, &srv.Auth, &srv.Harga, &srv.NamaServer, &srv.Quota, &srv.IPLimit, &srv.BatasCreateAkun, &srv.TotalCreateAkun)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &srv, nil
}

func (s *Store) IncrementServerCreate(id int64) error {
	_, err := s.DB.Exec(`UPDATE server SET total_create_akun = total_create_akun + 1 WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateServerField(id int64, field string, value any) error {
	allowed := map[string]bool{"harga": true, "nama_server": true, "domain": true, "auth": true, "quota": true, "iplimit": true, "batas_create_akun": true, "total_create_akun": true}
	if !allowed[field] {
		return fmt.Errorf("field tidak diizinkan")
	}
	_, err := s.DB.Exec(fmt.Sprintf(`UPDATE server SET %s = ? WHERE id = ?`, field), value, id)
	return err
}
