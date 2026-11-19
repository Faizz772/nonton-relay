# nonton-relay

Relay kecil (Go, tanpa dependency) untuk dipasang di VPS. Tujuannya: meneruskan permintaan API ke `nonton.blog`, karena IP datacenter (Vercel) diblokir Cloudflare. VPS dengan IP biasa tidak diblokir.

Relay hanya meneruskan path `/api/*` dan mewajibkan token rahasia. Bukan proxy terbuka.

## 1. Build di VPS

```bash
# di VPS (Ubuntu/Debian), pastikan Go terpasang
sudo apt update && sudo apt install -y golang-go
cd ~ && mkdir nonton-relay && cd nonton-relay
# salin main.go dan go.mod ke folder ini (scp/git), lalu:
go build -o nonton-relay .
```

## 2. Uji manual

```bash
RELAY_TOKEN=GANTI_TOKEN_PANJANG RELAY_ADDR=:8081 ./nonton-relay
# di terminal lain:
curl -s http://127.0.0.1:8081/health
curl -s -H "X-Relay-Token: GANTI_TOKEN_PANJANG" http://127.0.0.1:8081/api/anime | head -c 200
```

## 3. Jalankan sebagai service (systemd)

Buat `/etc/systemd/system/nonton-relay.service`:

```ini
[Unit]
Description=nonton.blog relay
After=network.target

[Service]
ExecStart=/root/nonton-relay/nonton-relay
Environment=RELAY_ADDR=127.0.0.1:8081
Environment=RELAY_TOKEN=GANTI_TOKEN_PANJANG
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now nonton-relay
sudo systemctl status nonton-relay
```

`RELAY_ADDR=127.0.0.1:8081` membuat relay hanya bisa diakses dari dalam VPS. Untuk diakses Vercel, relay harus terekspos lewat HTTPS.

## 4. Ekspos lewat HTTPS (pilih salah satu)

### A. Caddy reverse proxy (paling mudah, auto-HTTPS)

```bash
sudo apt install -y caddy
```

Edit `/etc/caddy/Caddyfile`:

```
relay.domainmu.com {
    reverse_proxy 127.0.0.1:8081
}
```

```bash
sudo systemctl reload caddy
```

Pastikan DNS `relay.domainmu.com` (A record) mengarah ke IP VPS dan port 80/443 terbuka.

### B. Cloudflare Tunnel (tanpa buka port, tanpa domain sendiri)

```bash
# install cloudflared lalu:
cloudflared tunnel login
cloudflared tunnel create nonton-relay
cloudflared tunnel route dns nonton-relay relay.domainmu.com
cloudflared tunnel run --url http://127.0.0.1:8081 nonton-relay
```

## 5. Set environment di Vercel

Di project `aniwatch` (Settings, Environment Variables, Production):

| Variabel | Nilai |
| --- | --- |
| `NONTON_BASE_URL` | `https://relay.domainmu.com/api` |
| `NONTON_RELAY_TOKEN` | `GANTI_TOKEN_PANJANG` (sama dengan `RELAY_TOKEN`) |

Lalu redeploy:

```bash
vercel --prod
```

## 6. Verifikasi

```bash
curl -s https://aniwatch-lilac.vercel.app/api/home | head -c 200
```

Bila muncul JSON berisi `banners`, relay tersambung.

## Catatan keamanan

- `RELAY_TOKEN` harus panjang dan acak. Tanpa token, relay menolak (401).
- Batasi firewall VPS: hanya buka 80/443 (Caddy) atau tutup semua inbound dan pakai Cloudflare Tunnel.
- Relay hanya meneruskan `/api/*` GET ke `nonton.blog`, tidak ke host lain (tidak bisa dipakai sebagai proxy umum).
