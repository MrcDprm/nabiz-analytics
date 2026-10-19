// Betiği sahte bir tarayıcı ortamında (node:vm) çalıştırıp gönderdiklerini kontrol eder.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { gzipSync } from "node:zlib";
import vm from "node:vm";

const source = readFileSync(new URL("../tracker/nabiz.js", import.meta.url), "utf8");

function browser({ href = "https://a.com/", referrer = "", dnt, ignore, webdriver, before, visibility = "visible", src = "https://nabiz.example.com/js/nabiz.js", api } = {}) {
  const sent = [];
  const listeners = {};
  const docListeners = {};
  const url = new URL(href);
  const location = {
    get href() { return url.href; },
    get pathname() { return url.pathname; },
    get hostname() { return url.hostname; },
    get protocol() { return url.protocol; },
  };
  const window = {
    innerWidth: 1280,
    addEventListener: (type, fn) => { listeners[type] = fn; },
    localStorage: { getItem: (k) => (k === "nabiz_ignore" && ignore ? "true" : null) },
    history: { pushState: (_s, _t, path) => { url.pathname = path; } },
  };
  const document = {
    referrer,
    visibilityState: visibility,
    currentScript: {
      src: new URL(src, href).href,
      getAttribute: (k) => ({ "data-site": "a.com", "data-api": api })[k] ?? null,
    },
    addEventListener: (type, fn) => { docListeners[type] = fn; },
    removeEventListener: (type) => { delete docListeners[type]; },
  };
  const navigator = {
    doNotTrack: dnt ? "1" : null,
    webdriver: !!webdriver,
    sendBeacon: (api, body) => { sent.push({ api, ...JSON.parse(body) }); return true; },
  };
  const ctx = { window, document, location, navigator, URL, JSON };
  window.window = window;
  if (before) before(window);
  vm.createContext(ctx);
  vm.runInContext(source, ctx);
  return {
    sent, window, url,
    navigate(path) { window.history.pushState({}, "", path); },
    back(path) { url.pathname = path; listeners.popstate(); },
    show() { document.visibilityState = "visible"; docListeners.visibilitychange(); },
  };
}

test("ilk sayfa görüntülemesini gönderir", () => {
  const b = browser({ href: "https://a.com/projects?utm_source=x", referrer: "https://google.com/" });
  assert.equal(b.sent.length, 1);
  assert.deepEqual(b.sent[0], {
    api: "https://nabiz.example.com/api/event",
    n: "pageview", u: "https://a.com/projects?utm_source=x", r: "https://google.com/", s: "a.com", w: 1280,
  });
});

test("proxy kurulumu: betik sitenin kendi adresinden, olaylar data-api'ye", () => {
  const b = browser({ href: "https://a.com/blog", src: "/_v/s.js", api: "/_v/e" });
  assert.equal(b.sent.length, 1);
  assert.equal(b.sent[0].api, "/_v/e");
});

test("SPA adres değişimini sayar, aynı adresi iki kez saymaz", () => {
  const b = browser();
  b.navigate("/hakkimda");
  b.navigate("/hakkimda");
  b.back("/");
  assert.deepEqual(b.sent.map((e) => new URL(e.u).pathname), ["/", "/hakkimda", "/"]);
});

test("özel olay ve özellikleri gönderir", () => {
  const b = browser();
  b.window.nabiz("Kayıt", { props: { paket: "pro" } });
  assert.equal(b.sent[1].n, "Kayıt");
  assert.deepEqual(b.sent[1].p, { paket: "pro" });
});

test("betikten önce kuyruğa alınan çağrıları gönderir", () => {
  const b = browser({
    before: (w) => {
      w.nabiz = function () { (w.nabiz.q = w.nabiz.q || []).push(arguments); };
      w.nabiz("Erken");
    },
  });
  assert.deepEqual(b.sent.map((e) => e.n), ["Erken", "pageview"]);
});

test("Do Not Track, localhost, otomasyon ve sahibin kendi ziyareti sayılmaz", () => {
  for (const opts of [
    { dnt: true },
    { href: "http://localhost:5173/" },
    { href: "http://127.0.0.1/" },
    { href: "file:///C:/site/index.html" },
    { webdriver: true },
    { ignore: true },
  ]) {
    assert.equal(browser(opts).sent.length, 0, JSON.stringify(opts));
  }
});

test("önceden yüklenen sayfa görünür olunca sayılır", () => {
  const b = browser({ visibility: "prerender" });
  assert.equal(b.sent.length, 0);
  b.show();
  assert.equal(b.sent.length, 1);
});

test("sıkıştırılmış betik 1 KB'tan küçük", { skip: !existsSync(new URL("../dist/nabiz.js", import.meta.url)) && "önce npm run build" }, () => {
  const built = readFileSync(new URL("../dist/nabiz.js", import.meta.url));
  const size = gzipSync(built, { level: 9 }).length;
  assert.ok(size < 1024, `gzip boyutu ${size} bayt`);
});
