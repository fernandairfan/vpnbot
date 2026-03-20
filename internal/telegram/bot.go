package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/fernandairfan/vpnbot/internal/config"
	"github.com/fernandairfan/vpnbot/internal/db"
	"github.com/fernandairfan/vpnbot/internal/panel"
	"github.com/fernandairfan/vpnbot/internal/payment"
)

type Bot struct {
	cfg        *config.Config
	store      *db.Store
	payment    *payment.Client
	panel      *panel.Client
	httpClient *http.Client
	baseAPI    string
	offset     int64
	waitTopup  map[int64]bool
}

type updatesResp struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

type sendResp struct {
	OK     bool `json:"ok"`
	Result struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

func New(cfg *config.Config, store *db.Store) *Bot {
	return &Bot{cfg: cfg, store: store, payment: payment.New(cfg.Payment.BaseURL, cfg.Payment.Token, cfg.Payment.MerchantID), panel: panel.New(), httpClient: &http.Client{Timeout: 60 * time.Second}, baseAPI: "https://api.telegram.org/bot" + cfg.BotToken, waitTopup: map[int64]bool{}}
}

func (b *Bot) Start(ctx context.Context) error {
	go b.runPollTopup(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		updates, err := b.getUpdates(ctx)
		if err != nil {
			log.Printf("getUpdates: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, upd := range updates.Result {
			if upd.UpdateID >= b.offset {
				b.offset = upd.UpdateID + 1
			}
			if upd.Message == nil || strings.TrimSpace(upd.Message.Text) == "" {
				continue
			}
			if err := b.handleMessage(upd.Message); err != nil {
				log.Printf("handleMessage: %v", err)
				_, _ = b.sendMessage(upd.Message.Chat.ID, "Terjadi kesalahan: "+err.Error())
			}
		}
	}
}

func (b *Bot) isAdmin(userID int64) bool {
	for _, id := range b.cfg.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (b *Bot) handleMessage(m *Message) error {
	_ = b.store.EnsureUser(m.From.ID)
	text := strings.TrimSpace(m.Text)
	if b.waitTopup[m.From.ID] && !strings.HasPrefix(text, "/") {
		return b.handleTopupAmount(m)
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil
	}
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/start", "/menu":
		_, err := b.sendMessage(m.Chat.ID, b.menuText())
		return err
	case "/saldo":
		bal, _ := b.store.GetBalance(m.From.ID)
		_, err := b.sendMessage(m.Chat.ID, fmt.Sprintf("Saldo kamu: Rp %d", bal))
		return err
	case "/topup":
		b.waitTopup[m.From.ID] = true
		_, err := b.sendMessage(m.Chat.ID, "Masukkan nominal topup. Contoh: 10000")
		return err
	case "/listserver":
		return b.cmdListServer(m.Chat.ID)
	case "/admin":
		if !b.isAdmin(m.From.ID) {
			_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
			return err
		}
		_, err := b.sendMessage(m.Chat.ID, b.adminHelp())
		return err
	case "/helpadmin":
		if !b.isAdmin(m.From.ID) {
			_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
			return err
		}
		_, err := b.sendMessage(m.Chat.ID, b.adminHelp())
		return err
	case "/addsaldo":
		return b.cmdAddSaldo(m, parts)
	case "/addserver":
		return b.cmdAddServer(m, parts)
	case "/editharga", "/editnama", "/editdomain", "/editauth", "/editlimitquota", "/editlimitip", "/editlimitcreate", "/edittotalcreate":
		return b.cmdEditServer(m, cmd, parts)
	case "/broadcast":
		return b.cmdBroadcast(m, strings.TrimSpace(strings.TrimPrefix(text, parts[0])))
	case "/createssh":
		return b.cmdCreateSSH(m, parts)
	case "/createvmess":
		return b.cmdCreateSimple(m, parts, "vmess")
	case "/createvless":
		return b.cmdCreateSimple(m, parts, "vless")
	case "/createtrojan":
		return b.cmdCreateSimple(m, parts, "trojan")
	case "/createshadowsocks":
		return b.cmdCreateSimple(m, parts, "shadowsocks")
	case "/renewssh":
		return b.cmdRenewSimple(m, parts, "ssh")
	case "/renewvmess":
		return b.cmdRenewSimple(m, parts, "vmess")
	case "/renewvless":
		return b.cmdRenewSimple(m, parts, "vless")
	case "/renewtrojan":
		return b.cmdRenewSimple(m, parts, "trojan")
	default:
		return nil
	}
}

func (b *Bot) menuText() string {
	return fmt.Sprintf("%s siap digunakan\n\nUser:\n/start\n/menu\n/saldo\n/topup\n/listserver\n\nCreate:\n/createssh <username> <password> <hari> <iplimit> <server_id>\n/createvmess <username> <hari> <quota_gb> <iplimit> <server_id>\n/createvless <username> <hari> <quota_gb> <iplimit> <server_id>\n/createtrojan <username> <hari> <quota_gb> <iplimit> <server_id>\n/createshadowsocks <username> <hari> <quota_gb> <iplimit> <server_id>\n\nRenew:\n/renewssh <username> <hari> <server_id>\n/renewvmess <username> <hari> <quota_gb> <server_id>\n/renewvless <username> <hari> <quota_gb> <server_id>\n/renewtrojan <username> <hari> <quota_gb> <server_id>", b.cfg.NamaStore)
}

func (b *Bot) adminHelp() string {
	return "Admin:\n/addsaldo <user_id> <jumlah>\n/addserver <domain> <auth> <harga> <nama_server> <quota> <iplimit> <batas_create>\n/editharga <server_id> <harga>\n/editnama <server_id> <nama>\n/editdomain <server_id> <domain>\n/editauth <server_id> <auth>\n/editlimitquota <server_id> <quota>\n/editlimitip <server_id> <iplimit>\n/editlimitcreate <server_id> <batas>\n/edittotalcreate <server_id> <total>\n/broadcast <pesan>"
}

func (b *Bot) cmdListServer(chatID int64) error {
	servers, err := b.store.ListServers()
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		_, err := b.sendMessage(chatID, "Belum ada server")
		return err
	}
	var sb strings.Builder
	sb.WriteString("List Server:\n")
	for _, s := range servers {
		sb.WriteString(fmt.Sprintf("ID:%d | %s | domain:%s | harga:%d | quota:%d | iplimit:%d | limitcreate:%d | total:%d\n", s.ID, s.NamaServer, s.Domain, s.Harga, s.Quota, s.IPLimit, s.BatasCreateAkun, s.TotalCreateAkun))
	}
	_, err = b.sendMessage(chatID, sb.String())
	return err
}

func (b *Bot) cmdAddSaldo(m *Message, parts []string) error {
	if !b.isAdmin(m.From.ID) {
		_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
		return err
	}
	if len(parts) != 3 {
		_, err := b.sendMessage(m.Chat.ID, "Format: /addsaldo <user_id> <jumlah>")
		return err
	}
	uid, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		_, e := b.sendMessage(m.Chat.ID, "user_id tidak valid")
		return e
	}
	amt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		_, e := b.sendMessage(m.Chat.ID, "jumlah tidak valid")
		return e
	}
	if err := b.store.AddSaldoAdmin(uid, amt); err != nil {
		return err
	}
	_, err = b.sendMessage(m.Chat.ID, "Saldo berhasil ditambahkan")
	return err
}

func (b *Bot) cmdAddServer(m *Message, parts []string) error {
	if !b.isAdmin(m.From.ID) {
		_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
		return err
	}
	if len(parts) < 8 {
		_, err := b.sendMessage(m.Chat.ID, "Format: /addserver <domain> <auth> <harga> <nama_server> <quota> <iplimit> <batas_create>")
		return err
	}
	harga, _ := strconv.ParseInt(parts[3], 10, 64)
	quota, _ := strconv.ParseInt(parts[5], 10, 64)
	iplimit, _ := strconv.ParseInt(parts[6], 10, 64)
	batas, _ := strconv.ParseInt(parts[7], 10, 64)
	err := b.store.AddServer(db.Server{Domain: parts[1], Auth: parts[2], Harga: harga, NamaServer: parts[4], Quota: quota, IPLimit: iplimit, BatasCreateAkun: batas, TotalCreateAkun: 0})
	if err != nil {
		return err
	}
	_, err = b.sendMessage(m.Chat.ID, "Server berhasil ditambahkan")
	return err
}

func (b *Bot) cmdEditServer(m *Message, cmd string, parts []string) error {
	if !b.isAdmin(m.From.ID) {
		_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
		return err
	}
	if len(parts) < 3 {
		_, err := b.sendMessage(m.Chat.ID, "Format salah")
		return err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		_, e := b.sendMessage(m.Chat.ID, "server_id tidak valid")
		return e
	}
	value := parts[2]
	field := map[string]string{"/editharga": "harga", "/editnama": "nama_server", "/editdomain": "domain", "/editauth": "auth", "/editlimitquota": "quota", "/editlimitip": "iplimit", "/editlimitcreate": "batas_create_akun", "/edittotalcreate": "total_create_akun"}[cmd]
	var storeValue any = value
	if field != "nama_server" && field != "domain" && field != "auth" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			_, e := b.sendMessage(m.Chat.ID, "nilai harus angka")
			return e
		}
		storeValue = n
	}
	if err := b.store.UpdateServerField(id, field, storeValue); err != nil {
		return err
	}
	_, err = b.sendMessage(m.Chat.ID, "Server berhasil diupdate")
	return err
}

func (b *Bot) cmdBroadcast(m *Message, msg string) error {
	if !b.isAdmin(m.From.ID) {
		_, err := b.sendMessage(m.Chat.ID, "Tidak ada izin")
		return err
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		_, err := b.sendMessage(m.Chat.ID, "Format: /broadcast <pesan>")
		return err
	}
	rows, err := b.store.DB.Query(`SELECT user_id FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			_, _ = b.sendMessage(uid, msg)
		}
	}
	_, err = b.sendMessage(m.Chat.ID, "Broadcast selesai")
	return err
}

func (b *Bot) cmdCreateSSH(m *Message, parts []string) error {
	if len(parts) != 6 {
		_, err := b.sendMessage(m.Chat.ID, "Format: /createssh <username> <password> <hari> <iplimit> <server_id>")
		return err
	}
	username, password := parts[1], parts[2]
	days, _ := strconv.ParseInt(parts[3], 10, 64)
	iplimit, _ := strconv.ParseInt(parts[4], 10, 64)
	serverID, _ := strconv.ParseInt(parts[5], 10, 64)
	srv, err := b.store.GetServer(serverID)
	if err != nil || srv == nil {
		_, e := b.sendMessage(m.Chat.ID, "Server tidak ditemukan")
		return e
	}
	if err := b.store.DeductBalance(m.From.ID, srv.Harga, fmt.Sprintf("create-ssh-%d-%d", m.From.ID, time.Now().UnixNano())); err != nil {
		_, e := b.sendMessage(m.Chat.ID, err.Error())
		return e
	}
	msg, err := b.panel.CreateSSH(srv, username, password, days, iplimit)
	if err != nil {
		return err
	}
	_ = b.store.IncrementServerCreate(serverID)
	_, err = b.sendMessage(m.Chat.ID, msg)
	return err
}

func (b *Bot) cmdCreateSimple(m *Message, parts []string, service string) error {
	if len(parts) != 6 {
		_, err := b.sendMessage(m.Chat.ID, fmt.Sprintf("Format: /create%s <username> <hari> <quota_gb> <iplimit> <server_id>", service))
		return err
	}
	username := parts[1]
	days, _ := strconv.ParseInt(parts[2], 10, 64)
	quota, _ := strconv.ParseInt(parts[3], 10, 64)
	iplimit, _ := strconv.ParseInt(parts[4], 10, 64)
	serverID, _ := strconv.ParseInt(parts[5], 10, 64)
	srv, err := b.store.GetServer(serverID)
	if err != nil || srv == nil {
		_, e := b.sendMessage(m.Chat.ID, "Server tidak ditemukan")
		return e
	}
	if err := b.store.DeductBalance(m.From.ID, srv.Harga, fmt.Sprintf("create-%s-%d-%d", service, m.From.ID, time.Now().UnixNano())); err != nil {
		_, e := b.sendMessage(m.Chat.ID, err.Error())
		return e
	}
	var msg string
	switch service {
	case "vmess":
		msg, err = b.panel.CreateVMess(srv, username, days, quota, iplimit)
	case "vless":
		msg, err = b.panel.CreateVLESS(srv, username, days, quota, iplimit)
	case "trojan":
		msg, err = b.panel.CreateTrojan(srv, username, days, quota, iplimit)
	case "shadowsocks":
		msg, err = b.panel.CreateShadowsocks(srv, username, days, quota, iplimit)
	}
	if err != nil {
		return err
	}
	_ = b.store.IncrementServerCreate(serverID)
	_, err = b.sendMessage(m.Chat.ID, msg)
	return err
}

func (b *Bot) cmdRenewSimple(m *Message, parts []string, service string) error {
	need := 4
	if service != "ssh" {
		need = 5
	}
	if len(parts) != need {
		_, err := b.sendMessage(m.Chat.ID, "Format renew salah")
		return err
	}
	username := parts[1]
	days, _ := strconv.ParseInt(parts[2], 10, 64)
	quota := int64(0)
	serverArg := 3
	if service != "ssh" {
		quota, _ = strconv.ParseInt(parts[3], 10, 64)
		serverArg = 4
	}
	serverID, _ := strconv.ParseInt(parts[serverArg], 10, 64)
	srv, err := b.store.GetServer(serverID)
	if err != nil || srv == nil {
		_, e := b.sendMessage(m.Chat.ID, "Server tidak ditemukan")
		return e
	}
	if err := b.store.DeductBalance(m.From.ID, srv.Harga, fmt.Sprintf("renew-%s-%d-%d", service, m.From.ID, time.Now().UnixNano())); err != nil {
		_, e := b.sendMessage(m.Chat.ID, err.Error())
		return e
	}
	var msg string
	switch service {
	case "ssh":
		msg, err = b.panel.RenewSSH(srv, username, days)
	case "vmess":
		msg, err = b.panel.RenewVMess(srv, username, days, quota)
	case "vless":
		msg, err = b.panel.RenewVLESS(srv, username, days, quota)
	case "trojan":
		msg, err = b.panel.RenewTrojan(srv, username, days, quota)
	}
	if err != nil {
		return err
	}
	_, err = b.sendMessage(m.Chat.ID, msg)
	return err
}

func (b *Bot) handleTopupAmount(m *Message) error {
	delete(b.waitTopup, m.From.ID)
	amount, err := strconv.ParseInt(strings.TrimSpace(m.Text), 10, 64)
	if err != nil || amount <= 0 {
		_, e := b.sendMessage(m.Chat.ID, "Nominal tidak valid")
		return e
	}
	unique := amount + int64((time.Now().UnixNano()%900)+100)
	msgID, err := b.sendQRIS(m.Chat.ID, unique)
	if err != nil {
		return err
	}
	p := db.PendingDeposit{UniqueCode: fmt.Sprintf("dep-%d-%d", m.From.ID, time.Now().UnixMilli()), UserID: m.From.ID, Amount: unique, OriginalAmount: amount, Timestamp: time.Now().UnixMilli(), Status: "pending", QRMessageID: msgID}
	if err := b.store.CreatePendingDeposit(p); err != nil {
		return err
	}
	_, err = b.sendMessage(m.Chat.ID, fmt.Sprintf("Topup dibuat\nNominal asli: Rp %d\nNominal bayar: Rp %d\nBiaya admin unik: Rp %d\nExpired: %d menit\n\nAuto cek dari AutoFT.", amount, unique, unique-amount, b.cfg.DepositExpiryMinutes))
	return err
}

func (b *Bot) runPollTopup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(b.cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.checkPending(ctx)
		}
	}
}

func (b *Bot) checkPending(ctx context.Context) {
	pending, err := b.store.ListPendingDeposits()
	if err != nil || len(pending) == 0 {
		return
	}
	trxs, err := b.payment.Transactions(ctx)
	if err != nil {
		log.Printf("payment: %v", err)
		return
	}
	for _, p := range pending {
		created := time.UnixMilli(p.Timestamp)
		if time.Since(created) > time.Duration(b.cfg.DepositExpiryMinutes)*time.Minute {
			_, _ = b.sendMessage(p.UserID, "Topup expired. Silakan /topup lagi.")
			_ = b.store.DeletePendingDeposit(p.UniqueCode)
			continue
		}
		for _, trx := range trxs {
			if !payment.Successful(trx.Status) || trx.Amount != p.Amount {
				continue
			}
			tt, err := payment.ParseTime(trx.Time)
			if err == nil && tt.Before(created.Add(-1*time.Minute)) {
				continue
			}
			ref := trx.ID
			if ref == "" {
				ref = p.UniqueCode
			}
			if err := b.store.AddTopup(p.UserID, p.OriginalAmount, p.Amount, ref); err != nil {
				continue
			}
			bal, _ := b.store.GetBalance(p.UserID)
			_, _ = b.sendMessage(p.UserID, fmt.Sprintf("Pembayaran berhasil\nDeposit: Rp %d\nTotal bayar: Rp %d\nSaldo sekarang: Rp %d", p.OriginalAmount, p.Amount, bal))
			if b.cfg.GroupID != 0 {
				_, _ = b.sendMessage(b.cfg.GroupID, fmt.Sprintf("Topup masuk\nUser: %d\nDeposit: Rp %d\nTotal: Rp %d\nRef: %s\nIssuer: %s", p.UserID, p.OriginalAmount, p.Amount, ref, trx.Issuer))
			}
			_ = b.store.DeletePendingDeposit(p.UniqueCode)
			break
		}
	}
}

func (b *Bot) getUpdates(ctx context.Context) (*updatesResp, error) {
	form := url.Values{}
	form.Set("timeout", "25")
	form.Set("offset", strconv.FormatInt(b.offset, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseAPI+"/getUpdates", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out updatesResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (b *Bot) sendMessage(chatID int64, text string) (int64, error) {
	payload := map[string]any{"chat_id": chatID, "text": text}
	body, _ := json.Marshal(payload)
	resp, err := b.httpClient.Post(b.baseAPI+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out sendResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Result.MessageID, nil
}

func (b *Bot) sendQRIS(chatID int64, amount int64) (int64, error) {
	png, err := qrcode.Encode(b.cfg.DataQRIS, qrcode.Medium, 300)
	if err != nil {
		return 0, err
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("qris-%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, png, 0600); err != nil {
		return 0, err
	}
	defer os.Remove(tmp)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	_ = writer.WriteField("caption", fmt.Sprintf("QRIS %s\nBayar: Rp %d", b.cfg.NamaStore, amount))
	fileWriter, err := writer.CreateFormFile("photo", filepath.Base(tmp))
	if err != nil {
		return 0, err
	}
	f, err := os.Open(tmp)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := io.Copy(fileWriter, f); err != nil {
		return 0, err
	}
	writer.Close()
	req, err := http.NewRequest(http.MethodPost, b.baseAPI+"/sendPhoto", &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out sendResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Result.MessageID, nil
}
