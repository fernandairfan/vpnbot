package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	merchantID string
	httpClient *http.Client
}

type Transaction struct {
	ID          string `json:"id"`
	Time        string `json:"time"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	PaymentType string `json:"payment_type"`
	Status      string `json:"status"`
	Issuer      string `json:"issuer"`
}

type response struct {
	Success bool `json:"success"`
	Data    struct {
		Transactions []Transaction `json:"transactions"`
	} `json:"data"`
	Message string `json:"message"`
}

func New(baseURL, token, merchantID string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, merchantID: merchantID, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Transactions(ctx context.Context) ([]Transaction, error) {
	body, _ := json.Marshal(map[string]any{"merchant_id": c.merchantID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transactions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 || !out.Success {
		return nil, fmt.Errorf("autoft error: %s", out.Message)
	}
	return out.Data.Transactions, nil
}

func ParseTime(v string) (time.Time, error) {
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, v, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", v)
}

func Successful(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "settlement", "success", "paid", "capture":
		return true
	default:
		return false
	}
}
