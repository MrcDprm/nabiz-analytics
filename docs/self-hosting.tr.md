[English](self-hosting.md) | Türkçe

# Nabız'ı kendi sunucuna kurmak

Nabız tek bir program dosyası ve bir SQLite veritabanı. Kendi sunucunda paket ya da sınır yoktur ve veri hiçbir zaman makinenden çıkmaz.

## Gerekenler

- Bir Linux sunucu (amd64 ya da arm64). Ayda milyonlarca sayfa görüntüleme için 1 işlemci ve 512 MB bellek yeter.
- Sunucuyu gösteren bir alan adı ya da alt alan adı, ör. `nabiz.example.com`.
- HTTPS için [Caddy](https://caddyserver.com) (ya da zaten kullandığın bir ters proxy).

## 1. Programı al

[Son sürümden](https://github.com/MrcDprm/nabiz-analytics/releases/latest) `nabiz-linux-amd64` ya da `nabiz-linux-arm64` dosyasını indir ya da kendin derle (Go 1.27 ve Node 24):

```sh
git clone https://github.com/MrcDprm/nabiz-analytics
cd nabiz-analytics/web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o nabiz ./cmd/nabiz
```

Panel ve takip betiği programın içine gömülü; başka bir şey kopyalamana gerek yok.

## 2. Kur

```sh
sudo useradd --system --home /var/lib/nabiz --shell /usr/sbin/nologin nabiz
sudo install -m 0755 nabiz /usr/local/bin/nabiz
sudo install -m 0644 deploy/nabiz.service deploy/nabiz-geoip.service deploy/nabiz-geoip.timer /etc/systemd/system/
sudo install -d -m 0700 /etc/nabiz
sudo install -m 0600 deploy/nabiz.env.example /etc/nabiz/nabiz.env
```

`/etc/nabiz/nabiz.env` dosyasında `NABIZ_BASE_URL` değerini kendi adresinle değiştir; diğer ayarlar dosyada açıklanıyor. Sonra başlat:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now nabiz nabiz-geoip.timer
sudo systemctl start nabiz-geoip   # ülke veritabanını şimdi bir kez indirir
```

Nabız `127.0.0.1:8081` adresini dinler; dışarıdan sadece ters proxy üzerinden ulaşılır.

## 3. Caddy ile HTTPS

`deploy/Caddyfile` dosyasını `/etc/caddy/Caddyfile` olarak kopyala (ya da kendi dosyana ekle), `nabiz.example.com` yerine kendi alan adını yaz ve Caddy'yi yeniden yükle:

```sh
sudo systemctl reload caddy
```

Caddy sertifikayı kendisi alır. `header_up X-Forwarded-For` satırı sadece betiği sitenin kendi alan adından sunuyorsan önemli; bkz. [proxy.tr.md](proxy.tr.md).

## 4. Hesabın ve ilk siten

```sh
sudo -u nabiz nabiz user add -data /var/lib/nabiz sen@example.com
```

Şifre iki kez sorulur ve ekranda görünmez. `https://nabiz.example.com` adresini aç, giriş yap, siteni ekle ve verdiği satırı sitenin her sayfasına koy:

```html
<script defer src="https://nabiz.example.com/js/nabiz.js" data-site="example.com"></script>
```

Sitende İçerik Güvenliği Politikası (CSP) varsa `script-src` ve `connect-src`'ye `https://nabiz.example.com` ekle.

## Ayarlar

Bütün ayarlar `/etc/nabiz/nabiz.env` dosyasında; değiştirince `sudo systemctl restart nabiz` çalıştır.

| Ayar | Ne yapar |
|---|---|
| `NABIZ_MODE` | `selfhost` (varsayılan): paket ve sınır yok, `NABIZ_SIGNUP=1` değilse hesap sadece komut satırından açılır. `cloud`: herkese açık kayıt, Ücretsiz paket sınırları. |
| `NABIZ_BASE_URL` | Senin adresin; e-postalardaki bağlantılarda kullanılır. `cloud` için zorunlu. |
| `NABIZ_SIGNUP` | `1` ise kendi sunucunda da herkese açık kayıt açılır. |
| `NABIZ_DEMO_SITE` | Tanıtım sayfasında canlı nabzı gösterilen herkese açık site (`cloud`). |
| `NABIZ_SMTP_*` | Doğrulama, şifre sıfırlama ve davet e-postaları. Yoksa e-postalar sadece log'a yazılır. 587 (STARTTLS) ve 465 (TLS) portları; şifresiz bağlantı reddedilir. |
| `NABIZ_IYZICO_*` | `cloud` modunda isteğe bağlı ücretli paketler; iyzico üye işyeri hesabı gerekir. Hepsi boşsa ödeme kapalıdır. |

## Yedekler

Nabız her gece veritabanının tutarlı bir kopyasını `/var/lib/nabiz/backup/nabiz-YYYY-AA-GG.db` olarak yazar ve son 7 yedeği tutar. Disk arızasına karşı bu klasörü sunucunun dışına da kopyala (rsync, restic, bir depolama alanı).

Geri yüklemek için: servisi durdur, `/var/lib/nabiz/nabiz.db` dosyasını bir yedekle değiştir (sahibi `nabiz`, izni 600) ve servisi başlat.

## Ülke verisi

Ülkeler ücretsiz [DB-IP IP to Country Lite](https://db-ip.com) veritabanından gelir (CC BY 4.0). Zamanlayıcı ayda bir günceller ve sunucuyu durdurmadan yeniden yükler. Bu dosya olmadan da Nabız çalışır; sadece ülkeler boş kalır.

## Güncelleme

`/usr/local/bin/nabiz` dosyasını yeni sürümle değiştir ve `sudo systemctl restart nabiz` çalıştır. Veritabanı değişiklikleri açılışta kendiliğinden uygulanır.

## İşe yarar komutlar

```sh
sudo -u nabiz nabiz site add -data /var/lib/nabiz -owner sen@example.com example.com
sudo -u nabiz nabiz geoip update -data /var/lib/nabiz
nabiz version
journalctl -u nabiz -f
```
