# SEO notes

Who we want to reach: people who run a website and want visitor stats without a cookie banner, without Google Analytics, or with their data kept in Turkey / on their own server.

## Keywords

### Turkish

**Broad (competitive)**
- web analitiği
- site istatistikleri
- ziyaretçi sayacı
- google analytics alternatifi

**Specific (long tail, main targets)**
- çerezsiz web analitiği
- çerez bandı olmadan site istatistikleri
- KVKK uyumlu web analitiği
- KVKK uyumlu google analytics alternatifi
- ücretsiz google analytics alternatifi
- kişisel veri toplamayan analitik
- açık kaynak web analitiği
- kendi sunucunda web analitiği kurulumu
- verisi Türkiye'de kalan analitik aracı
- plausible alternatifi türkçe

### English

**Broad (competitive)**
- web analytics
- website visitor counter
- google analytics alternative

**Specific (long tail, main targets)**
- cookie-free web analytics
- privacy-friendly google analytics alternative
- web analytics without cookie banner
- GDPR compliant analytics without cookies
- self-hosted web analytics single binary
- open source analytics Go SQLite
- lightweight analytics script under 1 KB
- plausible alternative self-hosted free

## Where they are used

| Place | Text |
|---|---|
| Landing `<title>` (TR) | Nabız: çerezsiz web analitiği, Google Analytics alternatifi |
| Landing `<title>` (EN) | Nabız: cookie-free web analytics, a Google Analytics alternative |
| Meta descriptions | `internal/server/seo.go` (`seoPages`), one per page and language |
| JSON-LD | `WebApplication` on `/` and `/en`, free offer, AGPL license, author |
| Language pages | `/`, `/how-it-works`, `/privacy`, `/terms` (TR) and `/en/...` (EN), linked with `hreflang` |
| `sitemap.xml` | the 8 public pages above with their language alternates |
| `robots.txt` | everything allowed except `/api/`; dashboards, login and settings pages carry `noindex` |
| Share image | `web/public/og-tr.png`, `web/public/og-en.png` (1200×630) |
| GitHub | repo description, topics and the first README paragraph |
| Release | "Nabız v1.0.0: cookie-free web analytics" and its notes |
| Portfolio | `/projects/nabiz` title, description and `SoftwareApplication` JSON-LD |

Self-hosted servers (`NABIZ_MODE=selfhost`) are never indexed: every page is `noindex`, `robots.txt` disallows everything and there is no sitemap.

## Google Search Console

1. Open [Search Console](https://search.google.com/search-console). If the **domain** property `miracdeprem.com` is already verified, it covers this subdomain too; use it and skip to step 3.
2. Otherwise add a **Domain** property `miracdeprem.com` and verify it with the TXT record Google gives (DNS settings at Vercel).
3. Sitemaps → submit `https://nabiz.miracdeprem.com/sitemap.xml`.
4. URL Inspection → request indexing for `https://nabiz.miracdeprem.com/` and `https://nabiz.miracdeprem.com/en`.
