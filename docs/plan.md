# Nabız — v1 planı

Çerezsiz, gizlilik dostu web analitiği. Tek Go dosyası + SQLite. Lisans AGPL-3.0. İki çalışma modu: **kendi sunucunda** (`-mode=selfhost`, ödeme yok) ve **barındırılan hizmet** (`-mode=cloud`, nabiz.miracdeprem.com, kayıt + ücretli paketler).

## Kapsam

Gelecek Planları yok; aşağıdakilerin hepsi v1.0.0'dan önce biter.

- **Takip:** <1 KB betik, sayfa görüntüleme, SPA adres değişimi, özel olay + olay özellikleri (props), Do Not Track, proxy rehberi (Vercel, Caddy, Nginx).
- **Gizlilik:** çerezsiz sayma (günlük tuz), IP diske yazılmaz, yerel GeoIP, "Nasıl çalışır" sayfası, KVKK aydınlatma metni taslağı.
- **Panel:** canlı ziyaretçi (nabız şeridi), özet metrikler + önceki dönem, zaman grafiği, sayfalar (en çok/giriş/çıkış), kaynaklar + UTM, ülke/cihaz/tarayıcı/OS, tıkla-süz, hedefler + dönüşüm, olay özellikleri kırılımı, huniler, CSV dışa aktarma.
- **Hesap:** herkese açık kayıt + e-posta doğrulaması, şifre sıfırlama, hesap ayarları, hesabı ve bütün veriyi silme (KVKK silme hakkı).
- **Siteler ve takım:** çoklu site, e-postayla takım daveti, roller (sahip / yönetici / izleyici), site başına herkese açık panel.
- **Ödeme (cloud modu, isteğe bağlı):** paketler, aylık kullanım sayacı, iyzico Abonelik ile ödeme (kart bilgisi bize hiç gelmez, iyzico'nun kendi formu), imzalı webhook, plan sayfası. iyzico anahtarları verilmezse ödeme kapalı çalışır: herkes Ücretsiz paketin sınırlarında, daha fazlası kendi sunucunda sınırsız.
- **Yasal sayfalar:** Gizlilik ve KVKK Aydınlatma, Kullanım Koşulları. Satış sözleşmeleri (Mesafeli Satış, Ön Bilgilendirme) sadece ödemeyi açan işletmeci için şablon olarak (docs).
- **Tanıtım sayfası:** ana sayfa, miracdeprem.com'un herkese açık canlı panelini gösterir (sahte ekran görüntüsü değil), fiyatlar, kurulum.
- **Sunucu:** bot eleme, Origin kontrolü, hız sınırı, gece yedeği, TR/EN, açık/koyu tema.

**Dış adımlar (kod biter, bunlar Miraç'ta):**
- nabiz.miracdeprem.com ücretsiz çalışır, ödeme kapalı (2026-10-19 kararı: şirket yok ve olmayacak; ücretli paket açılmayacak). Ödeme kodu açık kaynak olarak kalır; üye işyeri hesabı olan biri NABIZ_IYZICO_* ayarlarıyla açabilir.
- SMTP sağlayıcısı (ör. Brevo ücretsiz paket, günde 300 e-posta) ve alan adı için SPF/DKIM kaydı.

## Mimari

```
ziyaretçinin tarayıcısı                   Oracle VM (Zarf ile aynı makine)
┌──────────────────────┐   POST /api/event   ┌─────────────────────────────────────────┐
│ site + nabiz.js      │ ──────────────────► │ Caddy (HTTPS) → nabiz (Go, 127.0.0.1)   │
│ (<1 KB, çerez yok)   │   text/plain JSON   │  ├ ingest: doğrula → bot ele → UA çöz   │
└──────────────────────┘                     │  │   → GeoIP (bellekte) → hash → oturum │
                                             │  ├ stats API, CSV                        │
site sahibinin tarayıcısı                    │  ├ hesap, takım, e-posta (SMTP)          │
┌──────────────────────┐   /api/…            │  ├ ödeme (iyzico API + webhook)          │
│ panel (TS + Vite)    │ ◄─────────────────► │  ├ panel + tanıtım (web/dist gömülü)     │
└──────────────────────┘                     │  └ SQLite WAL (data/nabiz.db)            │
                                             └─────────────────────────────────────────┘
```

Klasörler:

```
cmd/nabiz/            main: bayraklar, sunucu; "nabiz user add" ve "nabiz geoip update" alt komutları
internal/store/       SQLite bağlantısı, şema/migration, sorgular
internal/salt/        günlük tuz: üret, gece yenile, eskisini sil
internal/ingest/      /api/event: doğrulama, bot eleme, UA, GeoIP, ziyaretçi hash'i, oturum, props
internal/stats/       özet, zaman serisi, kırılımlar, filtreler, canlı, huniler, CSV
internal/auth/        şifre (Argon2id), oturum çerezi, kayıt, doğrulama, sıfırlama
internal/team/        davetler, roller, yetki kontrolü
internal/mail/        SMTP gönderimi, TR/EN e-posta şablonları
internal/billing/     paketler, kullanım sayacı, iyzico istemcisi, webhook
internal/server/      yönlendirme, güvenlik başlıkları, CSP, hız sınırı
web/                  panel + tanıtım + yasal sayfalar (TypeScript + Vite), tracker/nabiz.js
deploy/               Caddyfile bloğu, nabiz.service
docs/                 plan, proxy rehberi, KVKK taslağı
```

## Ziyaretçi sayma (gizlilik çekirdeği)

- `visitor = ilk 8 bayt( SHA-256(günlük_tuz ‖ site_id ‖ IP ‖ User-Agent) )` → 64 bit tamsayı.
- Tuz: 32 rastgele bayt, UTC gece yarısı yenilenir, eskisi veritabanından silinir. Aynı kişi ertesi gün yeni biri gibi sayılır; bu bilinçli bir bedel (Plausible da aynı).
- IP sadece istek sırasında bellekte: hash ve ülke bulma. Hiçbir tabloya ya da log'a yazılmaz (Caddy access log kapalı).
- Ülke: DB-IP "IP to Country Lite" (mmdb, CC BY 4.0, atıf zorunlu). `nabiz geoip update` ile indirilir, systemd zamanlayıcısıyla ayda bir yenilenir; dosya repoya girmez.
- Do Not Track (`navigator.doNotTrack === "1"`) açıksa betik hiç istek atmaz.
- Proxy üzerinden gelen istekte istemci IP'si `X-Forwarded-For`'un ilk değerinden okunur. Bu değer taklit edilebilir; en kötü ihtimalle tekil ziyaretçi sayısı şişer, kişisel veri sızmaz. Rehberde yazılır.

## Oturum (ziyaret) mantığı

- Aynı `visitor` 30 dakikadan uzun ara vermediyse aynı oturum. Bellekte `visitor → (session_id, son_görülme)` haritası; 30 dk dokunulmayan girdi silinir. Yeniden başlatmada o anki oturumlar ikiye bölünebilir (kabul).
- Hemen çıkma = tek sayfa görüntülemeli oturum. Süre = oturumdaki son olay − ilk olay.
- Cihaz türü ekran genişliğinden: <576 mobil, <992 tablet, <1440 dizüstü, üstü masaüstü.

## Veri modeli (SQLite, WAL)

```sql
users         (id INTEGER PK, email TEXT UNIQUE, password_hash, email_verified_at, locale, created_at)
auth_sessions (token_hash BLOB PK, user_id, expires_at)
email_tokens  (token_hash BLOB PK, user_id, purpose /* verify|reset|email_change */, expires_at)
sites         (id TEXT PK /* UUID */, domain TEXT UNIQUE, public INTEGER, created_at)
memberships   (site_id, user_id, role /* owner|admin|viewer */, PK(site_id, user_id))
invites       (token_hash BLOB PK, site_id, email, role, invited_by, expires_at)
goals         (id TEXT PK, site_id, kind /* page|event */, value)
funnels       (id TEXT PK, site_id, name)
funnel_steps  (funnel_id, position, kind, value, PK(funnel_id, position))
salts         (day TEXT PK, value BLOB)                          -- sadece bugünkü
sessions      (id INTEGER PK, site_id, visitor INTEGER, started_at, ended_at, pageviews,
               entry_page, exit_page, referrer, utm_source, utm_medium, utm_campaign,
               country, device, browser, os)
events        (id INTEGER PK, site_id, ts, session_id, name /* pageview | özel */, pathname)
event_props   (event_id, site_id, key, value)
subscriptions (user_id PK, plan, status, iyzico_ref, current_period_end)
webhook_log   (event_id TEXT PK, received_at)                    -- aynı webhook iki kez işlenmez
-- indeksler: events(site_id, ts), sessions(site_id, started_at), event_props(site_id, key, value)
```

Önceki notta "ham olay + saatlik özet tablolar" yazıyordu. Değişiklik: sorgular `events` + `sessions` üstünden çalışır, çünkü tıkla-süz her boyut kombinasyonunu ister ve özet tablolar bunu karşılamaz.

Benchmark (adım 6, i7-10750H; tek sitede 30 güne yayılmış 1 milyon olay, ~320 bin ziyaret; `go test -run x -bench . ./internal/stats`): ilk hâlinde zaman serisi 2,3 s, sayfa filtresi 3,7 s, sayfa listesi 1,5 s'ydi. Üç düzeltme yapıldı, özet tabloya gerek kalmadı:
- Kapsayıcı indeksler: `sessions (site_id, started_at, visitor, pageviews, ended_at)`, `events (site_id, ts, name, pathname, visitor)` ve sayfa sırasına göre `events (site_id, name, pathname, ts, visitor)`; `events.visitor` sütunu sayfa sorgularını join'den kurtarır.
- Görüntüleme sayısı ziyaret satırındaki `pageviews` toplamından; sayfa/olay filtresi `EXISTS` yerine `IN (aralıktaki eşleşen olayların ziyaretleri)`.
- `uniq(x)`: `COUNT(DISTINCT x)` yerine Go map'iyle sayan SQLite fonksiyonu (internal/store/uniq.go), ~3 kat hızlı.

Sonuç: özet 150 ms, ülke filtresi 109 ms, sayfa filtresi 52 ms, zaman serisi 279 ms, kaynak 290 ms, sayfa listesi 384 ms. Bu, en büyük paketin üst sınırı; 100 bin olaylık sitede ~10 kat hızlı. Panel istekleri paralel atar.

Kullanım sayacı: bir hesabın sahibi olduğu bütün sitelerin o ayki `events` sayısı (sorgu + 10 dk önbellek).

## Roller

| Eylem | Sahip | Yönetici | İzleyici |
|---|---|---|---|
| Paneli görmek, CSV indirmek | ✓ | ✓ | ✓ |
| Hedef, huni, herkese açık panel ayarı | ✓ | ✓ | |
| Davet göndermek, rol değiştirmek | ✓ | ✓ (sahip hariç) | |
| Siteyi silmek, sahipliği devretmek | ✓ | | |

Ödeme sitenin sahibinin paketinden gelir. Yetki her istekte sunucuda `memberships` üzerinden kontrol edilir.

## Paketler (cloud modu, fiyatlar taslak)

| Paket | Aylık görüntüleme | Site | Takım üyesi | Fiyat (taslak) |
|---|---|---|---|---|
| Ücretsiz | 10 bin | 2 | 1 | 0 TL |
| Başlangıç | 100 bin | 10 | 3 | 149 TL/ay |
| Büyüme | 1 milyon | 50 | 10 | 399 TL/ay |

Limit aşılınca veri **atılmaz**: panelde ve e-postada uyarı çıkar, iki ay üst üste aşımda paket yükseltmesi istenir; yükseltilmezse yeni site eklenemez ama sayım sürer. Yıllık ödemede 2 ay ücretsiz. `-mode=selfhost` modunda paket ve limit yoktur.

**iyzico** seçildi çünkü hazır abonelik (tekrarlayan ödeme) API'si ve sandbox ortamı var. Ödeme iyzico'nun kendi formunda yapılır, kart verisi sunucumuza gelmez. Webhook'larda iyzico imzası doğrulanır, aynı olay iki kez işlenmez (`webhook_log`).

## HTTP uçları

| Uç | Kim | Not |
|---|---|---|
| `GET /js/nabiz.js` | herkes | betik, uzun önbellek |
| `POST /api/event` | betik | `text/plain` (CORS ön isteği yok), gövde ≤ 2 KB, Origin kayıtlı siteyle eşleşmeli, IP başına hız sınırı; her zaman 202 döner |
| `POST /api/signup`, `/api/verify`, `/api/login`, `/api/logout`, `/api/password/forgot`, `/api/password/reset` | herkes | hız sınırı; "bu e-posta kayıtlı mı" bilgisi sızdırılmaz |
| `GET/PATCH/DELETE /api/account` | giriş yapan | e-posta/şifre değiştirme, hesabı silme |
| `GET/POST /api/sites`, `PATCH/DELETE /api/sites/{id}` | rol | site ekle (snippet döner), ayarlar, sil |
| `…/sites/{id}/goals`, `…/funnels`, `…/members`, `…/invites` | rol | hedef, huni, üyeler, davetler |
| `POST /api/invites/{token}/accept` | davet edilen | hesap yoksa kayıt akışına yönlendirir |
| `GET /api/stats/{domain}/{summary,timeseries,breakdown,props,funnel,live}` | üye ya da public site | `period=day|7d|30d|month|12m|custom`, `filters=country:TR;page:/x` |
| `GET /api/stats/{domain}/export` | üye | seçili dönem ve filtrelerle ZIP (CSV'ler) |
| `GET /api/billing`, `POST /api/billing/checkout`, `POST /api/billing/cancel` | hesap | paket, kullanım, ödeme formu |
| `POST /api/billing/webhook` | iyzico | imza doğrulaması |

## Takip betiği

```html
<script defer src="https://nabiz.miracdeprem.com/js/nabiz.js" data-site="miracdeprem.com"></script>
```

- Sayfa görüntüleme, `history.pushState`/`popstate` ile SPA adres değişimi, `window.nabiz("Kayıt", { props: { paket: "pro" } })` ile özel olay.
- Gönderilen alanlar: `n` (olay adı), `u` (adres), `r` (referrer), `s` (site), `w` (ekran genişliği), `p` (props). Başka hiçbir şey yok.
- Props: olay başına en fazla 10, anahtar ≤ 32, değer ≤ 128 karakter. E-postaya benzeyen değerler sunucuda reddedilir (kişisel veri yanlışlıkla gönderilmesin).
- `navigator.sendBeacon`, yoksa `fetch(..., {keepalive:true})`. `localhost` ve `file:` adreslerinde çalışmaz.
- Boyut testi: sıkıştırılmış (minify) hâli 1 KB'tan küçük olmalı.
- Miraç'ın siteleri `*.miracdeprem.com` alt alanlarında, betiği doğrudan kullanır. Başka alan adları ve reklam engelleyiciler için `docs/proxy.md`: Vercel `rewrites`, Caddy, Nginx örnekleri.
- Her sitenin CSP'sine `script-src` ve `connect-src` olarak `https://nabiz.miracdeprem.com` eklenir.

## Huniler ve CSV

- Huni: 2-8 adım (sayfa ya da özel olay), aynı oturum içinde sırayla. Her adım için ulaşan ziyaretçi ve bir önceki adıma göre kayıp yüzdesi.
- CSV: `visitors.csv` (zaman serisi), `pages`, `entry_pages`, `exit_pages`, `sources`, `utm`, `countries`, `devices`, `browsers`, `os`, `goals`, `props`. `=`, `+`, `-`, `@` ile başlayan hücrelerin başına `'` eklenir (CSV/formül enjeksiyonu).

## Güvenlik kontrol listesi

- Parametreli SQL; istek gövdesinden sadece izin verilen alanlar alınır, uzunluklar sınırlanır (adres 512, referrer 512, olay adı 64).
- `/api/event` için CORS `*` değil: sadece kayıtlı sitenin Origin'i geri yansıtılır.
- Bot eleme: boş/headless UA, bilinen bot listesi (gömülü), Origin yok ya da kayıtlı değil.
- Şifre: Argon2id, en az 10 karakter + harf ve rakam, sık kullanılan şifre listesi reddedilir. Oturum çerezi `HttpOnly; Secure; SameSite=Strict`; token'lar veritabanında sadece hash olarak.
- E-posta doğrulanmadan site eklenemez. Doğrulama/sıfırlama/davet token'ları tek kullanımlık, 24 saat (davet 7 gün).
- Panel: sıkı CSP + Trusted Types (Zarf'taki gibi), `innerHTML` yok, POST'larda Origin kontrolü (CSRF).
- Site ve kayıt ID'leri UUID; yetki her istekte sunucuda. Panel adresi alan adını kullanır (gizli değil).
- Webhook imzası doğrulanmadan işlenmez. Gizli anahtarlar (iyzico, SMTP) ortam değişkeninde; repoda sadece `.env.example`.
- Production'da ayrıntılı hata yok; log'da IP ve e-posta yok.
- Hesap silme: kullanıcı, sahibi olduğu siteler ve bütün olayları silinir; abonelik iptal edilir.
- Gece `VACUUM INTO data/backup/nabiz-YYYY-MM-DD.db`, son 7 yedek tutulur.

## Geliştirme sırası

1. Go iskeleti: `go.mod`, bayraklar, SQLite + şema, `/healthz`.
2. Tuz ve ziyaretçi hash'i (+ testler).
3. Ingest: doğrulama, Origin, bot eleme, UA, GeoIP, oturum, props (+ testler).
4. Takip betiği + yerel deneme sayfası (+ boyut testi).
5. Kimlik: `nabiz user add`, giriş/çıkış, oturum çerezi.
6. İstatistik API: özet + önceki dönem, zaman serisi, kırılımlar, filtreler, canlı (+ fixture verili testler, benchmark).
7. Panel: yerleşim, nabız şeridi, grafik, kırılım listeleri, filtre, dönem seçici.
8. Siteler ve ayarlar: site ekle + snippet, herkese açık panel, hedefler.
9. Olay özellikleri kırılımı ve huniler (API + panel).
10. CSV dışa aktarma.
11. E-posta: SMTP, kayıt + doğrulama, şifre sıfırlama, hesap ayarları, hesap silme.
12. Takım: davet, roller, yetki testleri.
13. Ödeme: paketler, kullanım sayacı, limitler, iyzico sandbox, webhook, plan sayfası.
14. Tanıtım sayfası, "Nasıl çalışır", KVKK taslağı, yasal sayfalar, TR/EN, açık/koyu tema.
15. `docs/proxy.md` (Vercel, Caddy, Nginx) ve kendi sunucunda kurulum rehberi.
16. Yayın: Oracle'a kurulum, SMTP, 12 site + portfolyoya betik ve CSP.

Ardından yaşam döngüsü: test, README, son tarama, v1.0.0, SEO. Tahmin: 8-10 oturum.

## Tasarım planı

**Konu, kitle, iş:** küçük site sahibi (önce Miraç'ın kendisi) gündüz masada, "sitem bu hafta nasıldı, insanlar nereden geldi" sorusunu 10 saniyede cevaplamak istiyor. Yüzey türü: Operate (iş görme); tanıtım sayfası Persuade. Kelime "nabız" → hastane EKG kâğıdı: pembe milimetrik ızgara, üstünde mürekkep çizgisi. Görsel dil buradan gelir, ekran (monitör) estetiğinden değil.

**Renkler (açık tema, varsayılan):**

| Ad | Hex | Görev |
|---|---|---|
| EKG kâğıdı | `#FFFAF8` | zemin (pembe-beyaz; krem değil) |
| İnce ızgara | `#F5D3CE` | grafik ara çizgileri |
| Kalın ızgara | `#E8A79E` | grafik ana eksen çizgileri, bölüm ayırıcıları |
| Mürekkep | `#1D2B3F` | metin, grafik çizgisi (tükenmez kalem laciverti) |
| Kalp kırmızısı | `#C21F3A` | sadece canlı nabız, birincil buton, odak halkası |
| Kurşun kalem | `#6E6662` | ikincil metin, önceki dönem kesikli çizgisi |

Koyu tema "gece nöbeti": zemin `#1C1617`, ızgara `#3A2C2E` / `#5A4144`, metin `#F2E7E4`, kırmızı `#FF6B7D`. Parlama/halo yok.

**Yazı tipi:** tek aile, Atkinson Hyperlegible Next (Braille Institute, OFL-1.1, 150+ dil, 200-800 ağırlık). Okunabilirlik için tasarlanmış; hastane/erişilebilirlik dünyasından geliyor ve önceki projelerde kullanılmadı. Sayılar `tabular-nums`. Yerelde barındırılır (fontsource paketi); Türkçe harfler (ğ, ş, ı, İ) kurulumda kontrol edilir. Ölçek: 14 / 16 / 20 / 28 / 44.

**Panel yerleşimi:**

```
┌──────────────────────────────────────────────────────────────────┐
│ Nabız   miracdeprem.com ▾                  Son 7 gün ▾   TR  ◐   │
├──────────────────────────────────────────────────────────────────┤
│ 3 kişi şu an sitede  ──╮╭──────────╮╭────╮╭────────────────      │  ← nabız şeridi
├──────────────────────────────────────────────────────────────────┤
│ Ziyaretçi      Görüntüleme     Hemen çıkma     Ziyaret süresi    │
│ 1.204  +%12    3.880  +%4      %41  −%3        1 dk 32 sn        │
│ ┌ zaman grafiği: ızgara kâğıt, mürekkep çizgi, önceki dönem ┐    │
│ └ kesikli kurşun kalem ─────────────────────────────────────┘    │
│ Süzülüyor: Ülke = Türkiye ✕                       CSV indir      │
│ Sayfalar  [En çok][Giriş][Çıkış]   │ Kaynaklar  [Kaynak][UTM]    │
│ /                 ▓▓▓▓▓▓▓▓  820    │ google        ▓▓▓▓▓  410    │
│ /projects         ▓▓▓▓      402    │ linkedin.com  ▓▓     133    │
│ Ülkeler                            │ [Cihaz][Tarayıcı][İS]       │
│ Hedefler ve olay özellikleri       │ Huniler                     │
└──────────────────────────────────────────────────────────────────┘
```

Sola yaslı. Kırılım panelleri kart değil: kalın ızgara rengiyle ince çizgilerle ayrılmış alanlar, satır çubukları satırın arkasında hafif pembe dolgu. Metrik satırı bir monitördeki "vital değerler" satırı gibi tek sıra; tıklanan metrik grafiğe gelir.

**Tanıtım sayfası:** en üstte ürünün kendisi: miracdeprem.com'un gerçek, canlı herkese açık paneli (nabız şeridi atıyor) ve yanında tek cümle: "Çerez bandı olmadan sitenizi kimin ziyaret ettiğini görün." Altında "Ne topluyoruz / ne toplamıyoruz" iki sütunu, kurulum satırı (tek `<script>`), fiyat tablosu, kendi sunucunda kurma bağlantısı (GitHub). Üç sütunlu özellik ızgarası yok.

**İlkeler:**
- Cesaret tek yerde: canlı nabız şeridi. Son 5 dakikadaki her gerçek görüntüleme, EKG çizgisinde kendi zamanında bir QRS sıçraması olur; şerit yavaşça akar. `prefers-reduced-motion` açıksa durağan çizilir. Geri kalan her şey sakin.
- Izgara süs değil, bilgi: grafikteki ince/kalın çizgiler gerçek eksen aralıklarına denk gelir.
- Kırmızı nadir: sadece canlılık ve tek birincil eylem. Düşüş/artış yüzdeleri renk yerine işaret (+/−) ve kalınlıkla.
- Yazılar düz ve sade: "Bu hafta 1.204 ziyaretçi", "Henüz ziyaret yok. Betiği sitene ekle, ilk ziyaret burada görünecek."

**Varsayılanlara karşı kontrol (revizyon):**
- İlk akla gelen "koyu zemin + yeşil EKG çizgisi" monitör görünümüydü; bu yasaklı görünüm #2. Değiştirildi: kâğıt EKG, açık zemin.
- Metrik satırı "büyük sayı + küçük etiket" kalıbına yakın; burada sayılar ürünün kendisi olduğu için kalıyor, ama kutusuz, etiket sayıyla aynı satır ritminde ve etiketler büyük harf değil.
- Sekmeler gerçek sekme düğmesi olarak çizilir, "A · B · C" ara noktası yok.
- Tanıtım sayfasında "başlık solda, ekran görüntüsü sağda, iki buton" kalıbı yerine canlı panelin kendisi.
- Mono yazı tipi yok; kart, gölge, gradyan, kicker etiketi yok.
