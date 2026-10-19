// Command nabiz, Nabız analitik sunucusunu çalıştırır.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/geo"
	"github.com/MrcDprm/nabiz-analytics/internal/server"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

var version = "dev"

const usage = `nabiz: çerezsiz web analitiği

Kullanım:
  nabiz [serve] [-addr ADRES] [-data KLASÖR] [-mode selfhost|cloud] [-trust-proxy]
  nabiz geoip update [-data KLASÖR]   ülke veritabanını (DB-IP Lite) indirir ya da yeniler
  nabiz version

Her ayar ortam değişkeninden de gelebilir: NABIZ_ADDR, NABIZ_DATA, NABIZ_MODE, NABIZ_TRUST_PROXY=1.
`

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	// "geoip update" iki kelimelik komut; bayraklar ikinci kelimeden sonra gelir.
	sub := ""
	if cmd == "geoip" && len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	addr := fs.String("addr", env("NABIZ_ADDR", "127.0.0.1:8081"), "")
	data := fs.String("data", env("NABIZ_DATA", "data"), "")
	mode := fs.String("mode", env("NABIZ_MODE", string(server.ModeSelfhost)), "")
	trustProxy := fs.Bool("trust-proxy", os.Getenv("NABIZ_TRUST_PROXY") == "1", "")
	_ = fs.Parse(args)

	switch cmd {
	case "version":
		fmt.Println("nabiz", version)
	case "geoip":
		if sub != "update" || fs.NArg() != 0 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		client := &http.Client{Timeout: 5 * time.Minute}
		if err := geo.Update(ctx, client, geo.DefaultBaseURL, geoPath(*data), time.Now()); err != nil {
			log.Fatalf("nabiz: geoip: %v", err)
		}
		fmt.Println("Ülke veritabanı güncellendi. Çalışan sunucuya SIGHUP gönderince yeni dosyayı yükler.")
	case "serve":
		m := server.Mode(*mode)
		if m != server.ModeSelfhost && m != server.ModeCloud {
			fmt.Fprintln(os.Stderr, "nabiz: -mode selfhost ya da cloud olmalı")
			os.Exit(2)
		}
		cfg := server.Config{Mode: m, TrustProxy: *trustProxy, Version: version}
		if err := serve(cfg, *addr, *data); err != nil {
			log.Fatalf("nabiz: %v", err)
		}
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func geoPath(dataDir string) string { return filepath.Join(dataDir, "geoip.mmdb") }

func serve(cfg server.Config, addr, dataDir string) error {
	// 0700: veritabanını sadece sunucuyu çalıştıran kullanıcı okuyabilsin.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(dataDir, "nabiz.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hs := &http.Server{
		Addr:              addr,
		Handler:           server.New(cfg, st),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second, // CSV dışa aktarma bile bunun altında biter
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    16 << 10,
	}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	log.Printf("nabiz %s (%s) %s adresinde dinliyor", version, cfg.Mode, addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("nabiz: kapanıyor")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
