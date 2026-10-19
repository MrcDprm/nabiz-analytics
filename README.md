English | [Türkçe](README.tr.md)

# Nabız

Cookie-free, privacy-friendly web analytics that runs as a single Go binary with SQLite. No consent banner needed, no personal data stored, and the data stays on your own server.

> **Status:** planning (draft README).

## Features (planned v1)

**Tracking**
- Tiny dependency-free tracking script (under 1 KB): page views, custom events, SPA route changes.
- Respects Do Not Track.
- Served from the site's own domain so ad blockers don't drop it.

**Privacy**
- No cookies, no local storage, no fingerprint kept.
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

**Accounts and sites**
- Multiple sites per account.
- Optional public dashboard per site.

**Server**
- Bot and fake request filtering, Origin check against registered sites, rate limiting.
- Local GeoIP database (no third-party lookup).
- Raw events plus hourly rollup tables in SQLite (WAL).

## Screenshots

_Coming soon._

## Tech Stack

- **Server:** Go, SQLite (WAL)
- **Dashboard:** TypeScript, Vite
- **Tracker:** plain JavaScript
- **Hosting:** Caddy, systemd

## Installation

_Coming soon._

## What I Learned

_Coming soon._

## Future Plans

- Team invitations and roles.
- CSV export.
- Hosted paid plan.
