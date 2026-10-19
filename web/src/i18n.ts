// Arayüz metinleri (TR/EN). Seçim tarayıcıda hatırlanır; ilk açılışta tarayıcı diline bakılır.

export type Lang = "tr" | "en";

const tr = {
  loading: "Yükleniyor…",
  errorGeneric: "Bir şey ters gitti. Sayfayı yenileyip tekrar deneyin.",
  errorOffline: "Sunucuya ulaşılamadı. Bağlantınızı kontrol edin.",
  login: "Giriş yap",
  logout: "Çıkış yap",
  email: "E-posta",
  password: "Şifre",
  loginTitle: "Panele giriş",
  loginBusy: "Giriş yapılıyor…",
  sites: "Siteler",
  sitesTitle: "Sitelerin",
  noSites: "Henüz site yok. Sunucuda nabiz site add komutuyla ilk siteni ekle.",
  notFound: "Bu site yok ya da görme izniniz yok.",
  themeToggle: "Açık ve koyu tema arasında geç",
  langToggle: "Switch to English",
  period: "Dönem",
  "period.day": "Bugün",
  "period.7d": "Son 7 gün",
  "period.30d": "Son 30 gün",
  "period.month": "Bu ay",
  "period.12m": "Son 12 ay",
  "period.custom": "Tarih seç…",
  from: "Başlangıç",
  to: "Bitiş",
  apply: "Uygula",
  liveOne: "kişi şu an sitede",
  liveMany: "kişi şu an sitede",
  liveNote: "Son 5 dakika. Her sıçrama bir sayfa görüntüleme.",
  liveLabel: "Canlı ziyaretçi",
  visitors: "Ziyaretçi",
  pageviews: "Görüntüleme",
  bounce_rate: "Hemen çıkma",
  visit_length: "Ziyaret süresi",
  visits: "Ziyaret",
  metricGroup: "Grafikte gösterilecek ölçü",
  points: "puan",
  previous: "Önceki dönem",
  filtering: "Süzülüyor:",
  removeFilter: "Filtreyi kaldır",
  clearFilters: "Hepsini kaldır",
  pages: "Sayfalar",
  "tab.page": "En çok",
  "tab.entry": "Giriş",
  "tab.exit": "Çıkış",
  sources: "Kaynaklar",
  "tab.source": "Kaynak",
  "tab.utm_source": "UTM kaynak",
  "tab.utm_medium": "UTM ortam",
  "tab.utm_campaign": "UTM kampanya",
  countries: "Ülkeler",
  devices: "Cihazlar",
  "tab.device": "Cihaz",
  "tab.browser": "Tarayıcı",
  "tab.os": "İşletim sistemi",
  events: "Olaylar",
  "col.page": "Sayfa",
  "col.source": "Nereden",
  "col.country": "Ülke",
  "col.device": "Cihaz",
  "col.event": "Olay",
  "col.visitors": "Ziyaretçi",
  direct: "Doğrudan",
  unknown: "Bilinmiyor",
  "device.mobile": "Mobil",
  "device.tablet": "Tablet",
  "device.laptop": "Dizüstü",
  "device.desktop": "Masaüstü",
  empty: "Bu dönemde veri yok.",
  emptyEvents: "Henüz özel olay yok. Sitende nabiz(\"Kayıt\") ile gönderebilirsin.",
  more: "Daha fazla göster",
  noData: "Henüz ziyaret yok. Betiği sitene ekle; ilk ziyaret burada görünecek.",
  footerPrivacy: "Nabız çerez kullanmaz ve kişisel veri saklamaz.",
  footerGeo: "IP to Country Lite by DB-IP",
  filterHint: "Bu değere göre süz",
  loginWrong: "E-posta ya da şifre yanlış.",
  loginLimited: "Çok fazla deneme yaptınız. Bir dakika sonra tekrar deneyin.",
  public: "Herkese açık",
  noUtm: "UTM yok",
  other: "Diğer",
  backToSites: "Siteler",
  "dim.page": "Sayfa",
  "dim.entry": "Giriş sayfası",
  "dim.exit": "Çıkış sayfası",
  "dim.source": "Kaynak",
  "dim.utm_source": "UTM kaynak",
  "dim.utm_medium": "UTM ortam",
  "dim.utm_campaign": "UTM kampanya",
  "dim.country": "Ülke",
  "dim.device": "Cihaz",
  "dim.browser": "Tarayıcı",
  "dim.os": "İşletim sistemi",
  "dim.event": "Olay",
  chartLabel: "Zaman grafiği",
};

type Key = keyof typeof tr;

const en: Record<Key, string> = {
  loading: "Loading…",
  errorGeneric: "Something went wrong. Refresh the page and try again.",
  errorOffline: "Can't reach the server. Check your connection.",
  login: "Log in",
  logout: "Log out",
  email: "Email",
  password: "Password",
  loginTitle: "Log in to your dashboard",
  loginBusy: "Logging in…",
  sites: "Sites",
  sitesTitle: "Your sites",
  noSites: "No sites yet. Add your first one on the server with nabiz site add.",
  notFound: "This site doesn't exist or you don't have access to it.",
  themeToggle: "Switch between light and dark theme",
  langToggle: "Türkçeye geç",
  period: "Period",
  "period.day": "Today",
  "period.7d": "Last 7 days",
  "period.30d": "Last 30 days",
  "period.month": "This month",
  "period.12m": "Last 12 months",
  "period.custom": "Pick dates…",
  from: "From",
  to: "To",
  apply: "Apply",
  liveOne: "person on the site now",
  liveMany: "people on the site now",
  liveNote: "Last 5 minutes. Each spike is one page view.",
  liveLabel: "Live visitors",
  visitors: "Visitors",
  pageviews: "Page views",
  bounce_rate: "Bounce rate",
  visit_length: "Visit duration",
  visits: "Visits",
  metricGroup: "Metric shown in the chart",
  points: "pts",
  previous: "Previous period",
  filtering: "Filtered by:",
  removeFilter: "Remove filter",
  clearFilters: "Clear all",
  pages: "Pages",
  "tab.page": "Top",
  "tab.entry": "Entry",
  "tab.exit": "Exit",
  sources: "Sources",
  "tab.source": "Source",
  "tab.utm_source": "UTM source",
  "tab.utm_medium": "UTM medium",
  "tab.utm_campaign": "UTM campaign",
  countries: "Countries",
  devices: "Devices",
  "tab.device": "Device",
  "tab.browser": "Browser",
  "tab.os": "Operating system",
  events: "Events",
  "col.page": "Page",
  "col.source": "From",
  "col.country": "Country",
  "col.device": "Device",
  "col.event": "Event",
  "col.visitors": "Visitors",
  direct: "Direct",
  unknown: "Unknown",
  "device.mobile": "Mobile",
  "device.tablet": "Tablet",
  "device.laptop": "Laptop",
  "device.desktop": "Desktop",
  empty: "No data for this period.",
  emptyEvents: "No custom events yet. Send one from your site with nabiz(\"Signup\").",
  more: "Show more",
  noData: "No visits yet. Add the script to your site; the first visit will show up here.",
  footerPrivacy: "Nabız uses no cookies and stores no personal data.",
  footerGeo: "IP to Country Lite by DB-IP",
  filterHint: "Filter by this value",
  loginWrong: "Wrong email or password.",
  loginLimited: "Too many attempts. Try again in a minute.",
  public: "Public",
  noUtm: "No UTM",
  other: "Other",
  backToSites: "Sites",
  "dim.page": "Page",
  "dim.entry": "Entry page",
  "dim.exit": "Exit page",
  "dim.source": "Source",
  "dim.utm_source": "UTM source",
  "dim.utm_medium": "UTM medium",
  "dim.utm_campaign": "UTM campaign",
  "dim.country": "Country",
  "dim.device": "Device",
  "dim.browser": "Browser",
  "dim.os": "Operating system",
  "dim.event": "Event",
  chartLabel: "Time chart",
};

const dict: Record<Lang, Record<Key, string>> = { tr, en };

function initial(): Lang {
  try {
    const saved = localStorage.getItem("nabiz-lang");
    if (saved === "tr" || saved === "en") return saved;
  } catch {
    /* gizli pencere vb. */
  }
  return navigator.language.toLowerCase().startsWith("tr") ? "tr" : "en";
}

export let lang: Lang = initial();
document.documentElement.lang = lang;

export function setLang(l: Lang) {
  lang = l;
  document.documentElement.lang = l;
  try {
    localStorage.setItem("nabiz-lang", l);
  } catch {
    /* tercih hatırlanmaz, sorun değil */
  }
}

export function t(key: Key): string {
  return dict[lang][key];
}

export function tryT(key: string): string | undefined {
  return (dict[lang] as Record<string, string>)[key];
}

export const locale = () => (lang === "tr" ? "tr-TR" : "en-GB");
