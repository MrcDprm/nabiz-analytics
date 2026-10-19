[English](proxy.md) | Türkçe

# Betiği sitenin kendi alan adından sunmak

Bazı reklam engelleyiciler bilinen analitik adreslerine giden istekleri düşürür. Betiği ve isteklerini sitenin kendi alan adından sunarsan sitene giden diğer isteklerden farkı kalmaz ve çalışmaya devam eder. Bu isteğe bağlı; normal `<script>` satırı bunsuz da çalışır.

Fikir: sitendeki iki yol Nabız sunucuna yönlendirilir.

| Sitende | Yönlendirildiği yer |
|---|---|
| `/_v/s.js` | `https://nabiz.example.com/js/nabiz.js` |
| `/_v/e` | `https://nabiz.example.com/api/event` |

Yol adlarını sıradan seç; içinde "analytics", "stats" ya da "track" geçenlerin engellenme ihtimali daha yüksek. Sayfalarında da şu satırı kullan (`data-api` betiğe nereye göndereceğini söyler):

```html
<script defer src="/_v/s.js" data-site="example.com" data-api="/_v/e"></script>
```

Her şey aynı adresten geldiği için sitenin CSP'sine Nabız adresini eklemen artık gerekmez.

## Vercel

`vercel.json`:

```json
{
  "rewrites": [
    { "source": "/_v/s.js", "destination": "https://nabiz.example.com/js/nabiz.js" },
    { "source": "/_v/e", "destination": "https://nabiz.example.com/api/event" }
  ]
}
```

## Netlify

`_redirects`:

```
/_v/s.js  https://nabiz.example.com/js/nabiz.js  200
/_v/e     https://nabiz.example.com/api/event    200
```

## Caddy

```
example.com {
	handle /_v/s.js {
		rewrite * /js/nabiz.js
		reverse_proxy https://nabiz.example.com {
			header_up Host nabiz.example.com
		}
	}
	handle /_v/e {
		rewrite * /api/event
		reverse_proxy https://nabiz.example.com {
			header_up Host nabiz.example.com
		}
	}
	# ... sitenin geri kalanı
}
```

## Nginx

```nginx
location = /_v/s.js {
    proxy_pass https://nabiz.example.com/js/nabiz.js;
    proxy_set_header Host nabiz.example.com;
    proxy_ssl_server_name on;
}
location = /_v/e {
    proxy_pass https://nabiz.example.com/api/event;
    proxy_set_header Host nabiz.example.com;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_ssl_server_name on;
}
```

## Ziyaretçinin adresi proxy'den nasıl geçer

Proxy arkasında Nabız'a bağlanan ziyaretçi değil proxy'dir. Proxy ziyaretçinin adresini `X-Forwarded-For` başlığının başına koyar. Nabız (`deploy/nabiz.service`'teki gibi `-trust-proxy` ile) ziyaretçiyi sayarken ve ülkesini bulurken bu ilk değeri kullanır. Nabız'ın önündeki Caddy de bu başlığı silmemeli, korumalı; `deploy/Caddyfile`'daki `trusted_proxies` satırı bunu yapar.

İlk değeri isteği gönderen herkes taklit edebilir. En kötü ihtimalle tekil ziyaretçi sayısı şişer ya da bir ülke yanlış görünür; kimsenin verisi açığa çıkmaz. Hız sınırı ise son değere, yani gerçekten bağlanan adrese bakar; bu yolla atlatılamaz. `trusted_proxies` olmadan da her şey çalışır, ama proxy'den gelen bütün ziyaretçiler tek kişi sayılır.

`Origin` başlığını tarayıcı koyar ve proxy'den değişmeden geçer; Nabız yine sadece kayıtlı sitenden gelen olayları kabul eder.
