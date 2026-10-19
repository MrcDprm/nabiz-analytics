package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/site"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func openStore(dataDir string) (*store.Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return store.Open(filepath.Join(dataDir, "nabiz.db"))
}

// readPassword şifreyi ekranda göstermeden iki kez sorar. Girdi bir terminal
// değilse (ör. "echo şifre | nabiz user add") tek satır okur.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("şifre okunamadı")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Şifre: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Şifre (tekrar): ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("şifreler aynı değil")
	}
	return string(first), nil
}

func addUser(dataDir, email string) error {
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := auth.NewService(st, nil)
	if err != nil {
		return err
	}
	password, err := readPassword()
	if err != nil {
		return err
	}
	u, err := svc.CreateUser(context.Background(), email, password, true)
	if err != nil {
		return err
	}
	fmt.Printf("Hesap açıldı: %s\n", u.Email)
	return nil
}

func addSite(dataDir, ownerEmail, domain string) error {
	if ownerEmail == "" {
		return errors.New("-owner E-POSTA gerekli")
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := auth.NewService(st, nil)
	if err != nil {
		return err
	}
	u, err := svc.UserByEmail(context.Background(), ownerEmail)
	if errors.Is(err, auth.ErrInvalidLogin) {
		return fmt.Errorf("%s ile bir hesap yok; önce nabiz user add", ownerEmail)
	}
	if err != nil {
		return err
	}
	s, err := site.Create(context.Background(), st, domain, u.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Site eklendi: %s\nSitenin <head> bölümüne ekleyin:\n  <script defer src=\"https://NABIZ_ADRESI/js/nabiz.js\" data-site=\"%s\"></script>\n", s.Domain, s.Domain)
	return nil
}
