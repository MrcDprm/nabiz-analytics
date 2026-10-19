English | [Türkçe](README.tr.md)

# Nabız

Cookie-free, privacy-friendly web analytics that runs as a single Go binary with SQLite. No consent banner needed, no personal data stored, and the data stays on your own server.

> **Status:** planning (draft README).

## Features (planned v1)

**Tracking**
- Tiny dependency-free tracking script (under 1 KB): page views, SPA route changes, custom events with properties.
- Respects Do Not Track.
- One line to install: a `<script>` tag with the site's domain.
- Proxy guide for Vercel, Caddy and Nginx, so the script loads from your own domain.

**Privacy**
- No cookies and nothing written to the browser; no fingerprint kept.
- Visitors are counted with `hash(daily salt + site + IP + user agent)`. The salt rotates every night and the old one is deleted, so visits can't be linked across days.
- The IP is used only in memory to find the country and is never written to disk.
- A "How it works / what we collect" page and a KVKK / GDPR notice template.

**Dashboard**
- Live visitors.
- Visitors, page views, bounce rate and visit duration, compared with the previous period.
- Time series chart.
- Top pages, entry pages and exit pages.
- Sources and UTM campaigns.
- Countries, devices, browsers and operating systems.
- Click any row to filter the whole dashboard.
- Goals and conversion rate (page or custom event).
- Custom event properties breakdown.
- Funnels.
- CSV export.

**Accounts and teams**
- Sign up with email verification, password reset, account deletion with all data.
- Multiple sites per account.
- Team invitations by email with owner, admin and viewer roles.
- Optional public dashboard per site.

**Hosted version and plans**
- The public instance is free for everyone, with a monthly usage meter and fair limits (10,000 page views, 2 sites).
- Self-hosted mode has no plans or limits.
- Optional paid plans through iyzico subscriptions for operators with a merchant account; card data never reaches the server, webhooks are signed. Off unless the iyzico keys are set.

**Server**
- Bot and fake request filtering, Origin check against registered sites, rate limiting.
- Local GeoIP database (no third-party lookup).
- Events and visits in SQLite (WAL), nightly backup.

## Screenshots

_Coming soon._

## Tech Stack

- **Server:** Go, SQLite (WAL)
- **Dashboard:** TypeScript, Vite
- **Tracker:** plain JavaScript
- **Payments:** iyzico
- **Hosting:** Caddy, systemd

## Installation

_Coming soon._

## What I Learned

_Coming soon._

## License

[AGPL-3.0](LICENSE)
