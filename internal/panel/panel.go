package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yourgithub/fansstorevpn-go-final/internal/db"
)

type Client struct {
	httpClient *http.Client
}

func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func validUsername(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return !strings.Contains(v, " ")
}

func (c *Client) doJSON(method, endpoint, auth string, payload any) (map[string]any, error) {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("format respon tidak valid: %s", string(body))
	}
	return out, nil
}

func getMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}
func getString(m map[string]any, path ...string) string {
	cur := m
	for i, p := range path {
		if i == len(path)-1 {
			if v, ok := cur[p].(string); ok {
				return v
			}
			return fmt.Sprintf("%v", cur[p])
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}
func getFloat(m map[string]any, path ...string) float64 {
	cur := m
	for i, p := range path {
		if i == len(path)-1 {
			switch v := cur[p].(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case int64:
				return float64(v)
			default:
				return 0
			}
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			return 0
		}
		cur = next
	}
	return 0
}

func (c *Client) CreateSSH(srv *db.Server, username, password string, exp, iplimit int64) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	endpoint := fmt.Sprintf("http://%s/vps/sshvpn", srv.Domain)
	out, err := c.doJSON(http.MethodPost, endpoint, srv.Auth, map[string]any{"expired": exp, "kuota": "0", "limitip": fmt.Sprintf("%d", iplimit), "password": password, "username": username})
	if err != nil {
		return "", err
	}
	if getFloat(getMap(out, "meta"), "code") != 200 {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ SSH berhasil dibuat\n\nHost: %s\nUsername: %s\nPassword: %s\nExpired: %s %s\nIP Limit: %d\nSSH WS: %s:80@%s:%s\nSSH SSL: %s:443@%s:%s\nDownload Config: http://%s:81/myvpn-config.zip", getString(d, "hostname"), getString(d, "username"), getString(d, "password"), getString(d, "exp"), getString(d, "time"), iplimit, getString(d, "hostname"), getString(d, "username"), getString(d, "password"), getString(d, "hostname"), getString(d, "username"), getString(d, "password"), getString(d, "hostname"))
	return msg, nil
}

func (c *Client) CreateVMess(srv *db.Server, username string, exp, quota, iplimit int64) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	endpoint := fmt.Sprintf("http://%s/vps/vmessall", srv.Domain)
	out, err := c.doJSON(http.MethodPost, endpoint, srv.Auth, map[string]any{"expired": exp, "kuota": fmt.Sprintf("%d", quota), "limitip": fmt.Sprintf("%d", iplimit), "username": username})
	if err != nil {
		return "", err
	}
	if getFloat(getMap(out, "meta"), "code") != 200 {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ VMess berhasil dibuat\n\nUsername: %s\nHost: %s\nUUID: %s\nExpired: %s (%s)\nQuota: %d GB\nIP Limit: %d\nTLS: %s\nNon TLS: %s\ngRPC: %s", getString(d, "username"), getString(d, "hostname"), getString(d, "uuid"), getString(d, "expired"), getString(d, "time"), quota, iplimit, getString(getMap(d, "link"), "tls"), getString(getMap(d, "link"), "none"), getString(getMap(d, "link"), "grpc"))
	return msg, nil
}

func (c *Client) CreateVLESS(srv *db.Server, username string, exp, quota, iplimit int64) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	endpoint := fmt.Sprintf("http://%s/vps/vlessall", srv.Domain)
	out, err := c.doJSON(http.MethodPost, endpoint, srv.Auth, map[string]any{"expired": exp, "kuota": fmt.Sprintf("%d", quota), "limitip": fmt.Sprintf("%d", iplimit), "username": username})
	if err != nil {
		return "", err
	}
	if getFloat(getMap(out, "meta"), "code") != 200 {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ VLESS berhasil dibuat\n\nUsername: %s\nHost: %s\nUUID: %s\nExpired: %s (%s)\nQuota: %d GB\nIP Limit: %d\nTLS: %s\nNon TLS: %s\ngRPC: %s", getString(d, "username"), getString(d, "hostname"), getString(d, "uuid"), getString(d, "expired"), getString(d, "time"), quota, iplimit, getString(getMap(d, "link"), "tls"), getString(getMap(d, "link"), "none"), getString(getMap(d, "link"), "grpc"))
	return msg, nil
}

func (c *Client) CreateTrojan(srv *db.Server, username string, exp, quota, iplimit int64) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	endpoint := fmt.Sprintf("http://%s/vps/trojanall", srv.Domain)
	out, err := c.doJSON(http.MethodPost, endpoint, srv.Auth, map[string]any{"expired": exp, "kuota": fmt.Sprintf("%d", quota), "limitip": fmt.Sprintf("%d", iplimit), "username": username})
	if err != nil {
		return "", err
	}
	if getFloat(getMap(out, "meta"), "code") != 200 {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ Trojan berhasil dibuat\n\nUsername: %s\nHost: %s\nPassword/UUID: %s\nExpired: %s (%s)\nQuota: %d GB\nIP Limit: %d\nTLS: %s\ngRPC: %s", getString(d, "username"), getString(d, "hostname"), getString(d, "uuid"), getString(d, "expired"), getString(d, "time"), quota, iplimit, getString(getMap(d, "link"), "tls"), getString(getMap(d, "link"), "grpc"))
	return msg, nil
}

func (c *Client) CreateShadowsocks(srv *db.Server, username string, exp, quota, iplimit int64) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	endpoint := fmt.Sprintf("http://%s:5888/createshadowsocks?user=%s&exp=%d&quota=%d&iplimit=%d&auth=%s", srv.Domain, url.QueryEscape(username), exp, quota, iplimit, url.QueryEscape(srv.Auth))
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if fmt.Sprintf("%v", out["status"]) != "success" {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ Shadowsocks berhasil dibuat\n\nUsername: %s\nDomain: %s\nNS: %s\nTLS: %s\ngRPC: %s\nPubKey: %s\nExpired: %s\nQuota: %s\nIP Limit: %s\nSave: https://%s:81/shadowsocks-%s.txt", getString(d, "username"), getString(d, "domain"), getString(d, "ns_domain"), getString(d, "ss_link_ws"), getString(d, "ss_link_grpc"), getString(d, "pubkey"), getString(d, "expired"), getString(d, "quota"), getString(d, "ip_limit"), getString(d, "domain"), getString(d, "username"))
	return msg, nil
}

func (c *Client) RenewSSH(srv *db.Server, username string, exp int64) (string, error) {
	return c.renewCommon(srv, username, exp, 0, "renewsshvpn", "SSH")
}
func (c *Client) RenewVMess(srv *db.Server, username string, exp, quota int64) (string, error) {
	return c.renewCommon(srv, username, exp, quota, "renewvmess", "VMess")
}
func (c *Client) RenewVLESS(srv *db.Server, username string, exp, quota int64) (string, error) {
	return c.renewCommon(srv, username, exp, quota, "renewvless", "VLESS")
}
func (c *Client) RenewTrojan(srv *db.Server, username string, exp, quota int64) (string, error) {
	return c.renewCommon(srv, username, exp, quota, "renewtrojan", "Trojan")
}

func (c *Client) renewCommon(srv *db.Server, username string, exp, quota int64, endpoint, label string) (string, error) {
	if !validUsername(username) {
		return "", fmt.Errorf("username tidak valid")
	}
	fullURL := fmt.Sprintf("http://%s/vps/%s/%s/%d", srv.Domain, endpoint, username, exp)
	out, err := c.doJSON(http.MethodPatch, fullURL, srv.Auth, map[string]any{"kuota": quota})
	if err != nil {
		return "", err
	}
	if getFloat(getMap(out, "meta"), "code") != 200 {
		return "", fmt.Errorf("%v", out)
	}
	d := getMap(out, "data")
	msg := fmt.Sprintf("✅ Renew %s berhasil\n\nUsername: %s\nQuota: %v\nDari: %s\nSampai: %s", label, getString(d, "username"), d["quota"], getString(d, "from"), getString(d, "to"))
	return msg, nil
}
