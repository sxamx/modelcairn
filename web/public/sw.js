const CACHE = "modelcairn-shell-v2";
const SHELL = ["/", "/manifest.webmanifest", "/icon.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil((async () => {
    const cache = await caches.open(CACHE);
    const index = await fetch("/", { cache: "no-store" });
    if (!index.ok) throw new Error("shell_index_unavailable");
    const html = await index.clone().text();
    const assets = [...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map((match) => match[1]);
    await cache.put("/", index);
    await cache.addAll([...SHELL.slice(1), ...new Set(assets)]);
  })());
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key)))));
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  const requestURL = new URL(event.request.url);
  if (event.request.method !== "GET" || requestURL.origin !== self.location.origin ||
      requestURL.pathname.startsWith("/api/") || requestURL.pathname.startsWith("/v1/") ||
      requestURL.pathname === "/healthz" || requestURL.pathname === "/readyz") return;
  event.respondWith(fetch(event.request).then((response) => {
    if (response.ok && response.type === "basic") {
      const copy = response.clone();
      caches.open(CACHE).then((cache) => cache.put(event.request, copy));
    }
    return response;
  }).catch(() => caches.match(event.request).then((cached) => cached || caches.match("/"))));
});
