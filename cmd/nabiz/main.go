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
	"strings"
	"syscall"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/billing"
	"github.com/MrcDprm/nabiz-analytics/internal/geo"
	"github.com/MrcDprm/nabiz-analytics/internal/ingest"
	"github.com/MrcDprm/nabiz-analytics/internal/mail"
	"github.com/MrcDprm/nabiz-analytics/internal/salt"
	"github.com/MrcDprm/nabiz-analytics/internal/server"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
	"github.com/MrcDprm/nabiz-analytics/web"
)

var version = "dev"

const usage = `nabiz: çerezsiz web analitiği

Kullanım:
  nabiz [serve] [-addr ADRES] [-data KLASÖR] [-mode selfhost|cloud] [-trust-proxy] [-base-url URL] [-signup]
  nabiz geoip update [-data KLASÖR]               ülke veritabanını (DB-IP Lite) indirir ya da yeniler
  nabiz user add [-data KLASÖR] E-POSTA           hesap açar (şifre ekranda görünmeden sorulur)
  nabiz site add [-data KLASÖR] -owner E-POSTA ALAN_ADI   site ekler
  nabiz version

Her ayar ortam değişkeninden de gelebilir: NABIZ_ADDR, NABIZ_DATA, NABIZ_MODE, NABIZ_TRUST_PROXY=1,
NABIZ_BASE_URL, NABIZ_SIGNUP=1.
-base-url, e-postalardaki bağlantıların adresidir (ör. https://nabiz.example.com).
-signup herkese açık kaydı açar (cloud modunda her zaman açık).
E-posta: NABIZ_SMTP_HOST, NABIZ_SMTP_PORT, NABIZ_SMTP_USER, NABIZ_SMTP_PASS, NABIZ_SMTP_FROM.
SMTP yoksa e-postalar log'a yazılır (geliştirme).
`

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	// "geoip update", "user add" gibi iki kelimelik komutlar; bayraklar ikinci kelimeden sonra gelir.
	sub := ""
	if (cmd == "geoip" || cmd == "user" || cmd == "site") && len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	addr := fs.String("addr", env("NABIZ_ADDR", "127.0.0.1:8081"), "")
	data := fs.String("data", env("NABIZ_DATA", "data"), "")
	mode := fs.String("mode", env("NABIZ_MODE", string(server.ModeSelfhost)), "")
	trustProxy := fs.Bool("trust-proxy", os.Getenv("NABIZ_TRUST_PROXY") == "1", "")
	owner := fs.String("owner", "", "")
	baseURL := fs.String("base-url", os.Getenv("NABIZ_BASE_URL"), "")
	signup := fs.Bool("signup", os.Getenv("NABIZ_SIGNUP") == "1", "")
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
	case "user", "site":
		if sub != "add" || fs.NArg() != 1 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		var err error
		if cmd == "user" {
			err = addUser(*data, fs.Arg(0))
		} else {
			err = addSite(*data, *owner, fs.Arg(0))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "nabiz:", err)
			os.Exit(1)
		}
	case "serve":
		m := server.Mode(*mode)
		if m != server.ModeSelfhost && m != server.ModeCloud {
			fmt.Fprintln(os.Stderr, "nabiz: -mode selfhost ya da cloud olmalı")
			os.Exit(2)
		}
		var billingIyz *billing.Iyzico
		var billingCodes billing.PlanCodes
		cfg := server.Config{Mode: m, TrustProxy: *trustProxy, Version: version, Tracker: web.Tracker(), Web: web.Files(),
			BaseURL: strings.TrimRight(*baseURL, "/"), Signup: *signup || m == server.ModeCloud}
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://" + *addr
		}
		smtpCfg, err := mail.FromEnv()
		if err != nil {
			log.Fatalf("nabiz: %v", err)
		}
		if smtpCfg != nil {
			cfg.Mail = smtpCfg
		}
		// Barındırılan hizmette e-postalar gerçekten gitmeli ve bağlantılar HTTPS olmalı.
		if m == server.ModeCloud && (smtpCfg == nil || !strings.HasPrefix(cfg.BaseURL, "https://")) {
			log.Fatal("nabiz: cloud modunda NABIZ_SMTP_* ayarları ve https:// ile başlayan -base-url gerekli")
		}
		if smtpCfg == nil {
			log.Print("nabiz: SMTP ayarı yok, e-postalar gönderilmeyip log'a yazılıyor (sadece geliştirme için)")
		}
		if m == server.ModeCloud {
			iyz, codes, err := iyzicoFromEnv()
			if err != nil {
				log.Fatalf("nabiz: %v", err)
			}
			billingIyz, billingCodes = iyz, codes
		}
		if err := serve(cfg, *addr, *data, billingIyz, billingCodes); err != nil {
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

// reloadGeoOnHUP: "nabiz geoip update" sonrası "systemctl reload nabiz" (SIGHUP)
// sunucuyu durdurmadan yeni ülke dosyasını yükler.
func reloadGeoOnHUP(ctx context.Context, l *geo.Locator) {
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	for {
		select {
		case <-ctx.Done():
			return
		case <-hup:
			if err := l.Reload(); err != nil {
				log.Printf("nabiz: geoip yeniden yüklenemedi: %v", err)
			} else {
				log.Print("nabiz: ülke veritabanı yeniden yüklendi")
			}
		}
	}
}

func serve(cfg server.Config, addr, dataDir string, iyz *billing.Iyzico, codes billing.PlanCodes) error {
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

	keeper, err := salt.New(ctx, st.W, nil)
	if err != nil {
		return err
	}
	locator, err := geo.Open(geoPath(dataDir))
	if err != nil {
		return err
	}
	if cfg.Web == nil {
		log.Print("nabiz: panel derlenmemiş, sadece API çalışıyor (cd web && npm run build)")
	}
	if cfg.Tracker == nil {
		log.Print("nabiz: takip betiği derlenmemiş, /js/nabiz.js 404 dönecek (cd web && npm run build)")
	}
	if !locator.Loaded() {
		log.Print("nabiz: ülke veritabanı yok, ülkeler boş kalacak (nabiz geoip update)")
	}
	go reloadGeoOnHUP(ctx, locator)

	rec := ingest.NewRecorder(st, keeper, locator, nil)
	authSvc, err := auth.NewService(st, nil)
	if err != nil {
		return err
	}
	if iyz != nil {
		cfg.Billing = billing.New(st, iyz, codes, nil)
	}
	srv := server.New(cfg, st, rec, authSvc)
	go rec.Run(ctx)
	go srv.Run(ctx)

	hs := &http.Server{
		Addr:              addr,
		Handler:           srv,
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

// iyzicoFromEnv, barındırılan hizmetin ödeme ayarlarını ortam değişkenlerinden okur.
// Anahtarlar asla bayrakla verilmez (komut satırı diğer kullanıcılara görünebilir).
//
//	NABIZ_IYZICO_API_KEY, NABIZ_IYZICO_SECRET_KEY, NABIZ_IYZICO_MERCHANT_ID
//	NABIZ_IYZICO_BASE_URL (varsayılan sandbox)
//	NABIZ_IYZICO_PLAN_STARTER_MONTHLY, _STARTER_YEARLY, _GROWTH_MONTHLY, _GROWTH_YEARLY
func iyzicoFromEnv() (*billing.Iyzico, billing.PlanCodes, error) {
	iyz := &billing.Iyzico{
		BaseURL:    env("NABIZ_IYZICO_BASE_URL", billing.SandboxURL),
		APIKey:     os.Getenv("NABIZ_IYZICO_API_KEY"),
		SecretKey:  os.Getenv("NABIZ_IYZICO_SECRET_KEY"),
		MerchantID: os.Getenv("NABIZ_IYZICO_MERCHANT_ID"),
	}
	if iyz.APIKey == "" || iyz.SecretKey == "" || iyz.MerchantID == "" {
		return nil, nil, errors.New("cloud modunda NABIZ_IYZICO_API_KEY, NABIZ_IYZICO_SECRET_KEY ve NABIZ_IYZICO_MERCHANT_ID gerekli")
	}
	codes := billing.PlanCodes{}
	for _, plan := range []string{billing.Starter, billing.Growth} {
		for _, interval := range []string{"monthly", "yearly"} {
			key := "NABIZ_IYZICO_PLAN_" + strings.ToUpper(plan) + "_" + strings.ToUpper(interval)
			code := os.Getenv(key)
			if code == "" {
				return nil, nil, errors.New(key + " gerekli (iyzico panelindeki fiyatlandırma planı referans kodu)")
			}
			codes[plan+"/"+interval] = code
		}
	}
	if iyz.BaseURL == billing.SandboxURL {
		log.Print("nabiz: iyzico sandbox (deneme) ortamı kullanılıyor; gerçek ödeme alınmaz")
	}
	return iyz, codes, nil
}
