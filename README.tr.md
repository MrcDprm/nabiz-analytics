[English](README.md) | Türkçe

# Nabız

Tek Go dosyası ve SQLite ile çalışan, çerezsiz ve gizlilik dostu web analitiği. Çerez onay bandı gerekmez, kişisel veri saklanmaz ve veri kendi sunucunuzda kalır.

> **Durum:** planlama (taslak README).

## Özellikler (v1 planı)

**Takip**
- 1 KB altında, bağımlılıksız takip betiği: sayfa görüntüleme, SPA adres değişimi, özellikli özel olaylar.
- Do Not Track ayarına uyar.
- Tek satırla kurulur: sitenin alan adını taşıyan bir `<script>` etiketi.
- Betiğin kendi alan adınızdan yüklenmesi için Vercel, Caddy ve Nginx proxy rehberi.

**Gizlilik**
- Çerez yok, tarayıcıya hiçbir şey yazılmaz, parmak izi saklanmaz.
- Ziyaretçiler `hash(günlük tuz + site + IP + tarayıcı)` ile sayılır. Tuz her gece yenilenir ve eskisi silinir, böylece ziyaretler günler arasında birbirine bağlanamaz.
- IP sadece bellekte ülkeyi bulmak için kullanılır ve diske hiç yazılmaz.
- "Nasıl çalışır / ne topluyoruz" sayfası ve KVKK / GDPR aydınlatma metni taslağı.

**Panel**
- Canlı ziyaretçi.
- Ziyaretçi, görüntüleme, hemen çıkma oranı ve ziyaret süresi (önceki dönemle kıyaslı).
- Zaman grafiği.
- En çok görüntülenen, giriş ve çıkış sayfaları.
- Kaynaklar ve UTM kampanyaları.
- Ülkeler, cihazlar, tarayıcılar ve işletim sistemleri.
- Herhangi bir satıra tıklayarak bütün paneli süzme.
- Hedefler ve dönüşüm oranı (sayfa ya da özel olay).
- Özel olay özelliklerinin kırılımı.
- Huniler.
- CSV dışa aktarma.

**Hesap ve takım**
- E-posta doğrulamalı kayıt, şifre sıfırlama, hesabı bütün verisiyle silme.
- Hesap başına birden çok site.
- E-postayla takım daveti; sahip, yönetici ve izleyici rolleri.
- Site başına isteğe bağlı herkese açık panel.

**Barındırılan hizmet**
- Ücretsiz ve ücretli paketler, aylık kullanım sayacı.
- iyzico aboneliğiyle ödeme; kart bilgisi sunucuya hiç gelmez.
- Kendi sunucunuzda kurulumda paket ve limit yoktur.

**Sunucu**
- Bot ve sahte istek eleme, kayıtlı sitelere göre Origin kontrolü, hız sınırı.
- Yerel GeoIP veritabanı (üçüncü taraf sorgusu yok).
- Olaylar ve ziyaretler SQLite'ta (WAL), her gece yedek.

## Ekran Görüntüleri

_Yakında._

## Kullanılan Teknolojiler

- **Sunucu:** Go, SQLite (WAL)
- **Panel:** TypeScript, Vite
- **Takip betiği:** sade JavaScript
- **Ödeme:** iyzico
- **Barındırma:** Caddy, systemd

## Kurulum ve Çalıştırma

_Yakında._

## Öğrendiklerim

_Yakında._

## Lisans

[AGPL-3.0](LICENSE)
