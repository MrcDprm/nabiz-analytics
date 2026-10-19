// Nabız takip betiği. Çerez yok, tarayıcıya hiçbir şey yazmaz.
// Kurulum: <script defer src="https://nabiz.example.com/js/nabiz.js" data-site="example.com"></script>
(function () {
  var w = window, d = document, l = location, n = navigator;
  var script = d.currentScript;
  if (!script) return;
  var site = script.getAttribute("data-site");
  // Proxy kurulumunda (docs/proxy.md) olaylar sitenin kendi adresine gider.
  var api = script.getAttribute("data-api") || new URL(script.src).origin + "/api/event";

  function off() {
    // Do Not Track açıksa hiç istek atmayız.
    if (n.doNotTrack == "1" || w.doNotTrack == "1") return true;
    // Yerel geliştirme ve otomasyon araçları sayılmaz.
    if (/^localhost$|^127(\.\d+){3}$|^\[::1\]$/.test(l.hostname) || l.protocol == "file:") return true;
    if (n.webdriver) return true;
    // Site sahibi kendi ziyaretlerini saymamak için: localStorage.nabiz_ignore = "true"
    // (sadece okunur; betik hiçbir şey yazmaz).
    try {
      return w.localStorage.getItem("nabiz_ignore") == "true";
    } catch (e) {
      return false;
    }
  }

  // Gönderilen adres: sadece yol ve kaynak parametreleri. "#" sonrası (ör. Zarf'ta
  // şifre çözme anahtarı) ve diğer sorgu değerleri (ör. ?token=...) tarayıcıdan çıkmaz.
  var keep = ["utm_source", "utm_medium", "utm_campaign", "ref", "source"];
  function page_url() {
    var q = new URLSearchParams(l.search), out = new URLSearchParams();
    for (var i = 0; i < keep.length; i++) if (q.get(keep[i])) out.set(keep[i], q.get(keep[i]));
    var s = out.toString();
    return l.origin + l.pathname + (s ? "?" + s : "");
  }

  function send(name, options) {
    if (off()) return;
    var body = JSON.stringify({
      n: name,
      u: page_url(),
      r: d.referrer,
      s: site,
      w: w.innerWidth,
      p: options && options.props
    });
    // sendBeacon sayfa kapanırken bile gönderir ve gövdeyi text/plain yollar,
    // böylece CORS ön isteği olmaz. Yoksa ya da kuyruğa alamazsa fetch denenir.
    if (n.sendBeacon && n.sendBeacon(api, body)) return;
    if (w.fetch) {
      w.fetch(api, { method: "POST", body: body, keepalive: true, headers: { "Content-Type": "text/plain" } })
        .catch(function () {});
    }
  }

  var last;
  function page() {
    // Aynı sayfada sorgu ya da # değişti diye ikinci kez sayma.
    if (last === l.pathname) return;
    last = l.pathname;
    send("pageview");
  }

  // Tek sayfalı uygulamalar (React, Next.js...) sayfa değiştirirken yeniden
  // yüklenmez; history.pushState'i sarıp her adres değişiminde sayarız.
  var push = w.history && w.history.pushState;
  if (push) {
    w.history.pushState = function () {
      push.apply(this, arguments);
      page();
    };
    w.addEventListener("popstate", page);
  }

  // Betik yüklenmeden önce yapılan nabiz(...) çağrıları kuyrukta bekler.
  var queue = (w.nabiz && w.nabiz.q) || [];
  w.nabiz = send;
  for (var i = 0; i < queue.length; i++) send.apply(null, queue[i]);

  // Önceden yüklenen (prerender) sayfa, kullanıcı gerçekten açınca sayılır.
  if (d.visibilityState == "prerender") {
    d.addEventListener("visibilitychange", function once() {
      if (d.visibilityState == "visible") {
        d.removeEventListener("visibilitychange", once);
        page();
      }
    });
  } else {
    page();
  }
})();
