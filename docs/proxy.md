English | [Türkçe](proxy.tr.md)

# Serving the script from your own domain

Some ad blockers drop requests to known analytics addresses. If you serve the script and its requests from your site's own domain, they look like any other request to your site and keep working. This is optional; the normal `<script>` line works without it.

The idea: two paths on your site are forwarded to your Nabız server.

| On your site | Forwarded to |
|---|---|
| `/_v/s.js` | `https://nabiz.example.com/js/nabiz.js` |
| `/_v/e` | `https://nabiz.example.com/api/event` |

Pick neutral path names; anything with "analytics", "stats" or "track" in it is more likely to be blocked. Then use this line on your pages (`data-api` tells the script where to send):

```html
<script defer src="/_v/s.js" data-site="example.com" data-api="/_v/e"></script>
```

Your site's CSP no longer needs the Nabız address, because everything is same-origin.

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
	# ... the rest of your site
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

## How the visitor's address survives the proxy

Behind a proxy the connection to Nabız comes from the proxy, not the visitor. The proxy puts the visitor's address first in the `X-Forwarded-For` header, and Nabız (with `-trust-proxy`, as in `deploy/nabiz.service`) counts visitors and finds their country from that first value. Caddy in front of Nabız must keep the header instead of replacing it, which is what `trusted_proxies` in `deploy/Caddyfile` does.

The first value can be faked by anyone sending the request. At worst that inflates unique visitor counts or changes a country; nobody's data is exposed. Rate limiting uses the last value, the address that actually connected, so it can't be dodged this way. Without `trusted_proxies`, everything still works, but all visitors coming through the proxy count as one.

The `Origin` header is set by the browser and passes through the proxy unchanged, so Nabız still accepts events only from your registered site.
