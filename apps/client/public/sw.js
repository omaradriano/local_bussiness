// Service worker of the admin PWA: opens without network and loads fast on repeat visits.
// scripts/precache.mjs fills VERSION and PRECACHE (every bundle of the build) after `expo export`.
// Pages: network first (always the latest deploy), cached index.html when offline.
// Bundles, fonts and images: their names carry a hash, so cache first.
// API, Supabase and S3 are other origins and always go to the network.
const VERSION = 'dev';
const PRECACHE = [];
const CACHE = `app-${VERSION}`;

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(['/', ...PRECACHE])).then(() => self.skipWaiting()));
});

// A new deploy has a new VERSION: drop the previous build's files.
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (req.method !== 'GET' || url.origin !== self.location.origin) return;

  if (req.mode === 'navigate') {
    // Every route is the same SPA index.html, cached under '/'.
    event.respondWith(
      fetch(req)
        .then((res) => {
          if (res.ok) {
            const copy = res.clone(); // before the page reads the body
            event.waitUntil(caches.open(CACHE).then((c) => c.put('/', copy)));
          }
          return res;
        })
        .catch(() => caches.match('/')),
    );
    return;
  }

  if (url.pathname.startsWith('/_expo/static/') || url.pathname.startsWith('/assets/') || url.pathname.startsWith('/icons/')) {
    event.respondWith(
      caches.match(req).then((hit) => hit || fetch(req).then((res) => {
        if (res.ok) {
          const copy = res.clone();
          event.waitUntil(caches.open(CACHE).then((c) => c.put(req, copy)));
        }
        return res;
      })),
    );
  }
});
