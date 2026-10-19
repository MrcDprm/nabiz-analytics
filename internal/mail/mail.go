// Package mail, hesap e-postalarını (doğrulama, şifre sıfırlama, e-posta değişikliği) gönderir.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// Sender bir e-postayı gönderir. Gövde düz metindir: HTML e-posta yok, böylece
// e-postaya hiçbir şekilde kod ya da izleme pikseli giremez.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogSender e-postayı göndermez, log'a yazar. Sadece geliştirme için: içinde
// doğrulama bağlantısı olduğu için yayında kullanılmamalı.
type LogSender struct{}

func (LogSender) Send(_ context.Context, to, subject, body string) error {
	log.Printf("e-posta (gönderilmedi, geliştirme modu)\nKime: %s\nKonu: %s\n%s", to, subject, body)
	return nil
}

// SMTP ayarları ortam değişkenlerinden gelir; şifre asla bayrakla verilmez
// (komut satırı, sunucudaki diğer kullanıcılara görünebilir).
type SMTP struct {
	Host, User, Pass, From string
	Port                   int
}

// FromEnv: NABIZ_SMTP_HOST, NABIZ_SMTP_PORT (587 ya da 465), NABIZ_SMTP_USER,
// NABIZ_SMTP_PASS, NABIZ_SMTP_FROM. Host yoksa nil döner.
func FromEnv() (*SMTP, error) {
	host := os.Getenv("NABIZ_SMTP_HOST")
	if host == "" {
		return nil, nil
	}
	port, err := strconv.Atoi(os.Getenv("NABIZ_SMTP_PORT"))
	if err != nil || (port != 587 && port != 465 && port != 25 && port != 2525) {
		return nil, errors.New("NABIZ_SMTP_PORT 587, 465, 2525 ya da 25 olmalı")
	}
	s := &SMTP{Host: host, Port: port, User: os.Getenv("NABIZ_SMTP_USER"), Pass: os.Getenv("NABIZ_SMTP_PASS"), From: os.Getenv("NABIZ_SMTP_FROM")}
	if s.From == "" || strings.ContainsAny(s.From, "\r\n") {
		return nil, errors.New("NABIZ_SMTP_FROM gerekli (ör. Nabız <bildirim@example.com>)")
	}
	return s, nil
}

// Build, e-postanın ham hâlini üretir. Konu RFC 2047 ile, gövde quoted-printable
// ile kodlanır (Türkçe karakterler bozulmasın). Başlıklara satır sonu giremez:
// alıcı ve konu "\r\n" içerirse reddedilir (başlık enjeksiyonu).
func Build(from, to, subject, body string, now time.Time) ([]byte, error) {
	if strings.ContainsAny(to+subject+from, "\r\n") {
		return nil, errors.New("e-posta başlığında satır sonu olamaz")
	}
	id := make([]byte, 12)
	rand.Read(id)
	domain := "nabiz.local"
	if at := strings.LastIndex(from, "@"); at >= 0 {
		domain = strings.TrimRight(from[at+1:], ">")
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	qp.Write([]byte(strings.ReplaceAll(body, "\n", "\r\n")))
	qp.Close()
	return b.Bytes(), nil
}

func (s *SMTP) Send(ctx context.Context, to, subject, body string) error {
	msg, err := Build(s.From, to, subject, body, time.Now())
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	if s.Port == 465 {
		// 465: bağlantı baştan TLS.
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.Port != 465 {
		// 587/25: STARTTLS zorunlu; şifreyi düz metin bağlantıda göndermeyiz.
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("SMTP sunucusu STARTTLS desteklemiyor")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Pass, s.Host)); err != nil {
			return err
		}
	}
	fromAddr := s.From
	if i := strings.LastIndex(fromAddr, "<"); i >= 0 {
		fromAddr = strings.TrimSuffix(fromAddr[i+1:], ">")
	}
	if err := c.Mail(fromAddr); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
