# FANSSTOREVPN Go Final

Versi Go siap pakai untuk:
- topup via AutoFT `POST /transactions`
- QRIS statis
- saldo user
- add server
- edit server
- create SSH/VMess/VLESS/Trojan/Shadowsocks
- renew SSH/VMess/VLESS/Trojan
- SQLite
- systemd service

## Persiapan GitHub
1. Buat repository baru di GitHub.
2. Upload semua isi project ini ke repository tersebut.
3. Clone di VPS memakai `git clone`.

## Instalasi cepat di VPS
```bash
git clone https://github.com/fernandairfan/vpnbot.git /opt/vpnbot
cd /opt/fansstorevpn-go-final
cp config.example.json config.json
nano config.json
/usr/local/bin/go mod tidy
/usr/local/bin/go build -o fansstorevpn ./cmd/fansstorevpn
cp fansstorevpn.service /etc/systemd/system/fansstorevpn.service
systemctl daemon-reload
systemctl enable fansstorevpn
systemctl start fansstorevpn
```

## Command utama user
- `/start`
- `/menu`
- `/saldo`
- `/topup`
- `/listserver`
- `/createssh <username> <password> <hari> <iplimit> <server_id>`
- `/createvmess <username> <hari> <quota_gb> <iplimit> <server_id>`
- `/createvless <username> <hari> <quota_gb> <iplimit> <server_id>`
- `/createtrojan <username> <hari> <quota_gb> <iplimit> <server_id>`
- `/createshadowsocks <username> <hari> <quota_gb> <iplimit> <server_id>`
- `/renewssh <username> <hari> <server_id>`
- `/renewvmess <username> <hari> <quota_gb> <server_id>`
- `/renewvless <username> <hari> <quota_gb> <server_id>`
- `/renewtrojan <username> <hari> <quota_gb> <server_id>`

## Command admin
- `/admin`
- `/helpadmin`
- `/addsaldo <user_id> <jumlah>`
- `/addserver <domain> <auth> <harga> <nama_server> <quota> <iplimit> <batas_create>`
- `/editharga <server_id> <harga>`
- `/editnama <server_id> <nama>`
- `/editdomain <server_id> <domain>`
- `/editauth <server_id> <auth>`
- `/editlimitquota <server_id> <quota>`
- `/editlimitip <server_id> <iplimit>`
- `/editlimitcreate <server_id> <batas>`
- `/edittotalcreate <server_id> <total>`
- `/broadcast <pesan>`

## Catatan
- Token AutoFT disimpan di `config.json`.
- Request AutoFT dikirim sebagai JSON object biasa, bukan string manual.
- Nama store default: `FANSSTOREVPN`.
