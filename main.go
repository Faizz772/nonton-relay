// Command nonton-relay adalah relay kecil untuk dipasang di VPS.
//
// Latar belakang: IP datacenter (Vercel) diblokir Cloudflare di depan
// nonton.blog. VPS dengan IP biasa tidak diblokir, jadi relay ini meneruskan
// permintaan /api/* ke nonton.blog/api/* lalu mengembalikan JSON apa adanya.
//
// Keamanan: setiap permintaan wajib menyertakan token rahasia lewat header
// "X-Relay-Token" atau query "token". Tanpa token, permintaan ditolak.
// Hanya path /api/* yang diteruskan, sehingga relay tidak menjadi proxy terbuka.
package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const upstreamBase = "https://nonton.blog/api"

var upstreamClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	token := strings.TrimSpace(os.Getenv("RELAY_TOKEN"))
	if token == "" {
		fmt.Fprintln(os.Stderr, "RELAY_TOKEN wajib diset (token rahasia relay)")
		os.Exit(1)
	}
	addr := strings.TrimSpace(os.Getenv("RELAY_ADDR"))
	if addr == "" {
		addr = ":8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok","relay":true}`)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "token relay tidak valid", http.StatusUnauthorized)
			return
		}
		relay(w, r)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("nonton-relay berjalan di %s -> %s", addr, upstreamBase)
	log.Fatal(srv.ListenAndServe())
}

// authorized memeriksa token dengan perbandingan waktu-konstan.
func authorized(r *http.Request, token string) bool {
	got := r.Header.Get("X-Relay-Token")
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// relay meneruskan GET ke upstream dan mengembalikan body apa adanya.
func relay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}

	target := upstreamBase + strings.TrimPrefix(r.URL.Path, "/api")

	// Teruskan semua query kecuali "token" agar tidak bocor ke upstream.
	query := r.URL.Query()
	query.Del("token")
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	// Header menyerupai browser agar diterima Cloudflare.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", "https://nonton.blog/")
	req.Header.Set("Origin", "https://nonton.blog")

	resp, err := upstreamClient.Do(req)
	if err != nil {
		http.Error(w, "relay gagal: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=120")
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 16<<20))
}
