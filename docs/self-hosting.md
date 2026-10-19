English | [Türkçe](self-hosting.tr.md)

# Self-hosting Nabız

Nabız is one program file and an SQLite database. On your own server there are no plans or limits, and the data never leaves your machine.

## What you need

- A Linux server (amd64 or arm64). 1 CPU and 512 MB of RAM are plenty for millions of page views a month.
- A domain or subdomain pointing at it, for example `nabiz.example.com`.
- [Caddy](https://caddyserver.com) for HTTPS (or any reverse proxy you already use).

## 1. Get the program

Download `nabiz-linux-amd64` or `nabiz-linux-arm64` from the [latest release](https://github.com/MrcDprm/nabiz-analytics/releases/latest), or build it yourself (Go 1.27 and Node 24):

```sh
git clone https://github.com/MrcDprm/nabiz-analytics
cd nabiz-analytics/web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o nabiz ./cmd/nabiz
```

The dashboard and the tracking script are built into the binary; there is nothing else to copy.

## 2. Install

```sh
sudo useradd --system --home /var/lib/nabiz --shell /usr/sbin/nologin nabiz
sudo install -m 0755 nabiz /usr/local/bin/nabiz
sudo install -m 0644 deploy/nabiz.service deploy/nabiz-geoip.service deploy/nabiz-geoip.timer /etc/systemd/system/
sudo install -d -m 0700 /etc/nabiz
sudo install -m 0600 deploy/nabiz.env.example /etc/nabiz/nabiz.env
```

Edit `/etc/nabiz/nabiz.env`: set `NABIZ_BASE_URL` to your address. The other settings are explained in the file. Then start it:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now nabiz nabiz-geoip.timer
sudo systemctl start nabiz-geoip   # downloads the country database once now
```

Nabız listens on `127.0.0.1:8081`, so it is reachable only through the reverse proxy.

## 3. HTTPS with Caddy

Copy `deploy/Caddyfile` to `/etc/caddy/Caddyfile` (or merge it into yours), replace `nabiz.example.com` with your domain and reload Caddy:

```sh
sudo systemctl reload caddy
```

Caddy gets the certificate by itself. The `trusted_proxies` line matters only if you serve the script through your site's own domain; see [proxy.md](proxy.md).

## 4. Your account and your first site

```sh
sudo -u nabiz nabiz user add -data /var/lib/nabiz you@example.com
```

The password is asked twice and never shown. Open `https://nabiz.example.com`, log in, add your site and copy the line it gives you into every page of the site:

```html
<script defer src="https://nabiz.example.com/js/nabiz.js" data-site="example.com"></script>
```

If your site has a Content Security Policy, add `https://nabiz.example.com` to `script-src` and `connect-src`.

## Settings

All settings live in `/etc/nabiz/nabiz.env`; run `sudo systemctl restart nabiz` after changing it.

| Setting | What it does |
|---|---|
| `NABIZ_MODE` | `selfhost` (default): no plans or limits, sign-up only from the command line unless `NABIZ_SIGNUP=1`. `cloud`: open sign-up with the free plan limits. |
| `NABIZ_BASE_URL` | Your address; used in email links. Required for `cloud`. |
| `NABIZ_SIGNUP` | `1` opens sign-up on a self-hosted server. |
| `NABIZ_DEMO_SITE` | A public site whose live pulse is shown on the landing page (`cloud`). |
| `NABIZ_SMTP_*` | Email for verification, password reset and invites. Without it, emails are only written to the log. Ports 587 (STARTTLS) and 465 (TLS); plain connections are refused. |
| `NABIZ_IYZICO_*` | Optional paid plans in `cloud` mode; needs an iyzico merchant account. All empty means payments are off. |

## Backups

Every night Nabız writes a consistent copy of the database to `/var/lib/nabiz/backup/nabiz-YYYY-MM-DD.db` and keeps the last 7. Copy that folder off the server (rsync, restic, a storage bucket) to be safe from disk failure.

To restore: stop the service, replace `/var/lib/nabiz/nabiz.db` with a backup (owner `nabiz`, mode 600) and start it again.

## Country data

Countries come from the free [DB-IP IP to Country Lite](https://db-ip.com) database (CC BY 4.0). The timer updates it once a month and reloads the server without downtime. Without it Nabız still works; countries are just empty.

## Upgrading

Replace `/usr/local/bin/nabiz` with the new version and run `sudo systemctl restart nabiz`. Database changes are applied automatically at start.

## Useful commands

```sh
sudo -u nabiz nabiz site add -data /var/lib/nabiz -owner you@example.com example.com
sudo -u nabiz nabiz geoip update -data /var/lib/nabiz
nabiz version
journalctl -u nabiz -f
```
