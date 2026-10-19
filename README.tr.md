[English](README.md) | Türkçe

# Nabız

Tek Go dosyası ve SQLite ile çalışan, çerezsiz ve gizlilik dostu web analitiği. Çerez onay bandı gerekmez, kişisel veri saklanmaz ve veri kendi sunucunuzda kalır.

> **Durum:** planlama (taslak README).

## Özellikler (v1 planı)

**Takip**
- 1 KB altında, bağımlılıksız takip betiği: sayfa görüntüleme, özel olay, SPA adres değişimi.
- Do Not Track ayarına uyar.
- Reklam engelleyiciler düşürmesin diye sitenin kendi alan adından sunulur.

**Gizlilik**
- Çerez, yerel depolama ya da saklanan parmak izi yok.
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

**Hesap ve siteler**
- Hesap başına birden çok site.
- Site başına isteğe bağlı herkese açık panel.

**Sunucu**
- Bot ve sahte istek eleme, kayıtlı sitelere göre Origin kontrolü, hız sınırı.
- Yerel GeoIP veritabanı (üçüncü taraf sorgusu yok).
- SQLite'ta (WAL) ham olaylar ve saatlik özet tabloları.

## Ekran Görüntüleri

_Yakında._

## Kullanılan Teknolojiler

- **Sunucu:** Go, SQLite (WAL)
- **Panel:** TypeScript, Vite
- **Takip betiği:** sade JavaScript
- **Barındırma:** Caddy, systemd

## Kurulum ve Çalıştırma

_Yakında._

## Öğrendiklerim

_Yakında._

## Gelecek Planları

- Takım daveti ve roller.
- CSV dışa aktarma.
- Barındırılan ücretli sürüm.
