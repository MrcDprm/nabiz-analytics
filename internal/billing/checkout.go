package billing

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"
)

// CheckoutLifetime: açık bir ödeme formunun saklanma süresi.
const CheckoutLifetime = time.Hour

// StartCheckout iyzico'da ödeme formu açar ve kim için açıldığını kaydeder.
// Dönen token, ödeme sayfasının adresinde kullanılır.
func (s *Service) StartCheckout(ctx context.Context, userID int64, plan, interval, callbackURL, locale string, cust Customer) (string, error) {
	if s.iyz == nil {
		return "", ErrPaymentsOff
	}
	code, ok := s.codes[plan+"/"+interval]
	if !Paid(plan) || !ValidInterval(interval) || !ok {
		return "", ErrUnknownPlan
	}
	if r, err := s.sub(ctx, userID); err != nil {
		return "", err
	} else if r != nil && r.status == "ACTIVE" {
		return "", ErrAlreadyActive
	}
	token, content, err := s.iyz.InitCheckout(ctx, code, callbackURL, locale, cust)
	if err != nil {
		return "", err
	}
	_, err = s.st.W.ExecContext(ctx, `INSERT INTO checkouts (token, user_id, plan, interval, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		token, userID, plan, interval, content, s.now().Unix())
	return token, err
}

// CheckoutContent, ödeme sayfası için formu döndürür; sadece formu açan kullanıcıya.
func (s *Service) CheckoutContent(ctx context.Context, token string, userID int64) (string, error) {
	var content string
	err := s.st.R.QueryRowContext(ctx, `SELECT content FROM checkouts WHERE token = ? AND user_id = ? AND created_at > ?`,
		token, userID, s.now().Add(-CheckoutLifetime).Unix()).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrBadCheckout
	}
	return content, err
}

// Complete, iyzico ödeme formundan döndükten sonra sonucu iyzico'ya sorar (geri
// dönüş isteğine güvenmez: tarayıcıdan geldiği için taklit edilebilir) ve aboneliği kaydeder.
func (s *Service) Complete(ctx context.Context, token string) (int64, error) {
	if s.iyz == nil {
		return 0, ErrPaymentsOff
	}
	var userID int64
	var plan, interval string
	err := s.st.R.QueryRowContext(ctx, `SELECT user_id, plan, interval FROM checkouts WHERE token = ?`, token).Scan(&userID, &plan, &interval)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrBadCheckout
	}
	if err != nil {
		return 0, err
	}
	sub, err := s.iyz.RetrieveCheckout(ctx, token)
	if err != nil {
		return userID, err
	}
	// iyzico'nun söylediği plan, bizim kaydettiğimizle aynı olmalı.
	gotPlan, gotInterval, ok := s.codes.lookup(sub.PricingPlanReferenceCode)
	if !ok || gotPlan != plan || gotInterval != interval || sub.Status != "ACTIVE" || sub.ReferenceCode == "" {
		return userID, ErrBadCheckout
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return userID, err
	}
	defer tx.Rollback()
	if err := upsert(ctx, tx, userID, plan, interval, sub, s.periodEnd(sub, interval), s.now()); err != nil {
		return userID, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM checkouts WHERE token = ?`, token); err != nil {
		return userID, err
	}
	s.forget(userID)
	return userID, tx.Commit()
}

// periodEnd: iyzico bitiş tarihi verdiyse o, vermediyse bugünden bir dönem sonrası.
func (s *Service) periodEnd(sub *Subscription, interval string) time.Time {
	if sub.EndDate > 0 {
		return time.UnixMilli(sub.EndDate).UTC()
	}
	if interval == "yearly" {
		return s.now().AddDate(1, 0, 0)
	}
	return s.now().AddDate(0, 1, 0)
}

func upsert(ctx context.Context, tx *sql.Tx, userID int64, plan, interval string, sub *Subscription, end, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO subscriptions (user_id, plan, interval, status, iyzico_ref, customer_ref, current_period_end, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET plan = excluded.plan, interval = excluded.interval, status = excluded.status,
			iyzico_ref = excluded.iyzico_ref, customer_ref = excluded.customer_ref,
			current_period_end = excluded.current_period_end, updated_at = excluded.updated_at`,
		userID, plan, interval, sub.Status, sub.ReferenceCode, sub.CustomerReferenceCode, end.Unix(), now.Unix())
	return err
}

// Cancel aboneliği iyzico'da iptal eder; ödenmiş dönem bitene kadar paket sürer.
func (s *Service) Cancel(ctx context.Context, userID int64) error {
	if s.iyz == nil {
		return ErrNoActive
	}
	r, err := s.sub(ctx, userID)
	if err != nil {
		return err
	}
	if r == nil || (r.status != "ACTIVE" && r.status != "UNPAID") {
		return ErrNoActive
	}
	if err := s.iyz.Cancel(ctx, r.ref); err != nil {
		return err
	}
	_, err = s.st.W.ExecContext(ctx, `UPDATE subscriptions SET status = 'CANCELED', updated_at = ? WHERE user_id = ?`, s.now().Unix(), userID)
	s.forget(userID)
	return err
}

// ChangePlan aktif aboneliği başka pakete taşır (aynı dönem).
func (s *Service) ChangePlan(ctx context.Context, userID int64, plan, interval string) error {
	if s.iyz == nil {
		return ErrPaymentsOff
	}
	r, err := s.sub(ctx, userID)
	if err != nil {
		return err
	}
	if r == nil || r.status != "ACTIVE" {
		return ErrNoActive
	}
	if interval != r.interval {
		return ErrSameInterval
	}
	code, ok := s.codes[plan+"/"+interval]
	if !Paid(plan) || !ok {
		return ErrUnknownPlan
	}
	if plan == r.plan {
		return nil
	}
	sub, err := s.iyz.Upgrade(ctx, r.ref, code)
	if err != nil {
		return err
	}
	if sub.Status == "" {
		sub.Status = "ACTIVE"
	}
	if sub.CustomerReferenceCode == "" {
		sub.CustomerReferenceCode = r.customer
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsert(ctx, tx, userID, plan, interval, sub, s.periodEnd(sub, interval), s.now()); err != nil {
		return err
	}
	s.forget(userID)
	return tx.Commit()
}

// HandleWebhook iyzico'nun aylık çekim bildirimini işler. İmza tutmazsa reddeder;
// aynı bildirim (iyziReferenceCode) ikinci kez gelirse sessizce yok sayar.
func (s *Service) HandleWebhook(ctx context.Context, e WebhookEvent, signature string) error {
	if s.iyz == nil || !s.iyz.VerifyWebhook(e, signature) {
		return ErrBadSignature
	}
	if e.IyziReferenceCode == "" {
		return ErrBadSignature
	}
	res, err := s.st.W.ExecContext(ctx, `INSERT INTO webhook_log (event_id, received_at) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		e.IyziReferenceCode, s.now().Unix())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // daha önce işlendi
	}
	var userID int64
	err = s.st.R.QueryRowContext(ctx, `SELECT user_id FROM subscriptions WHERE iyzico_ref = ?`, e.SubscriptionReferenceCode).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("webhook: bilinmeyen abonelik") // referans log'a yazılmaz
		return nil
	}
	if err != nil {
		return err
	}
	switch e.IyziEventType {
	case "subscription.order.success":
		// Yeni dönem sonunu iyzico'dan öğren; olmazsa bir dönem ekle.
		end := s.now().AddDate(0, 1, 0)
		var interval string
		s.st.R.QueryRowContext(ctx, `SELECT interval FROM subscriptions WHERE user_id = ?`, userID).Scan(&interval)
		if interval == "yearly" {
			end = s.now().AddDate(1, 0, 0)
		}
		if sub, err := s.iyz.GetSubscription(ctx, e.SubscriptionReferenceCode); err == nil && sub.EndDate > 0 {
			end = time.UnixMilli(sub.EndDate).UTC()
		}
		_, err = s.st.W.ExecContext(ctx, `UPDATE subscriptions SET status = 'ACTIVE', current_period_end = ?, updated_at = ? WHERE user_id = ?`,
			end.Unix(), s.now().Unix(), userID)
	case "subscription.order.failure":
		_, err = s.st.W.ExecContext(ctx, `UPDATE subscriptions SET status = 'UNPAID', updated_at = ? WHERE user_id = ? AND status = 'ACTIVE'`,
			s.now().Unix(), userID)
	}
	s.forget(userID)
	return err
}

// OnAccountDelete: hesap silinirken aktif abonelik iptal edilir (bir daha çekim olmasın).
func (s *Service) OnAccountDelete(ctx context.Context, userID int64) error {
	err := s.Cancel(ctx, userID)
	if errors.Is(err, ErrNoActive) {
		return nil
	}
	return err
}

// Sweep süresi dolmuş, tamamlanmamış ödeme formlarını ve 90 günden eski webhook kayıtlarını siler.
func (s *Service) Sweep(ctx context.Context) error {
	if _, err := s.st.W.ExecContext(ctx, `DELETE FROM checkouts WHERE created_at <= ?`, s.now().Add(-CheckoutLifetime).Unix()); err != nil {
		return err
	}
	_, err := s.st.W.ExecContext(ctx, `DELETE FROM webhook_log WHERE received_at <= ?`, s.now().AddDate(0, 0, -90).Unix())
	return err
}
