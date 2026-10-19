import { defineConfig } from "vite";

// Geliştirmede sayfa Vite'tan, API yerel nabiz sunucusundan gelir
// (go run ./cmd/nabiz -data data); yayında ikisini de Go sunucusu sunar.
const API = "http://127.0.0.1:8081";

// Panel adresleri alan adıdır (/example.com). Vite noktalı adresleri dosya sanıp
// 404 verir; dosya uzantısı olmayan bu adresleri index.html'e yönlendiriyoruz.
const assetExt = /\.(ts|js|css|svg|png|woff2?|json|map|html|ico)$/;

export default defineConfig({
  plugins: [
    {
      name: "nabiz-site-routes",
      configureServer(server) {
        server.middlewares.use((req, _res, next) => {
          const path = (req.url ?? "").split("?")[0] ?? "";
          if (/^(\/settings)?\/[a-z0-9.-]+\.[a-z0-9-]+\/?$/.test(path) && !assetExt.test(path)) req.url = "/index.html";
          next();
        });
      },
    },
  ],
  server: { proxy: { "/api": API, "/js": API } },
  build: {
    outDir: "dist",
    assetsInlineLimit: 0, // yazı tipleri dosya olarak: CSP data: adreslerine izin vermiyor
  },
});
