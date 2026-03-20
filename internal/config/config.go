package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Payment struct {
	BaseURL    string `json:"base_url"`
	MerchantID string `json:"merchant_id"`
	Token      string `json:"token"`
}

type Config struct {
	BotToken             string  `json:"bot_token"`
	AdminIDs             []int64 `json:"admin_ids"`
	NamaStore            string  `json:"nama_store"`
	GroupID              int64   `json:"group_id"`
	Port                 int     `json:"port"`
	DataQRIS             string  `json:"data_qris"`
	Payment              Payment `json:"payment"`
	PollIntervalSeconds  int     `json:"poll_interval_seconds"`
	DepositExpiryMinutes int     `json:"deposit_expiry_minutes"`
	DatabasePath         string  `json:"database_path"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.NamaStore == "" {
		cfg.NamaStore = "FANSSTOREVPN"
	}
	if cfg.Payment.BaseURL == "" {
		cfg.Payment.BaseURL = "https://gopay.autoftbot.com/api/backend"
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 15
	}
	if cfg.DepositExpiryMinutes <= 0 {
		cfg.DepositExpiryMinutes = 5
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "fansstorevpn.db"
	}
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("bot_token wajib diisi")
	}
	if cfg.Payment.MerchantID == "" {
		return nil, fmt.Errorf("payment.merchant_id wajib diisi")
	}
	if cfg.Payment.Token == "" {
		return nil, fmt.Errorf("payment.token wajib diisi")
	}
	return &cfg, nil
}
