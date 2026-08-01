// Service worker for WODL PWA.
//
// Strategy:
//   - Static assets are served stale-while-revalidate: the cached copy answers
//     immediately and a background fetch refreshes it.
//
//     The stylesheet and htmx are requested with a content hash in the query
//     string (see static.AssetURL), which is what makes that safe. Pages are
//     network-first, so without the hash a new build's markup would be paired
//     with the previous build's stylesheet for a load — and since Tailwind only
//     emits the classes present in the templates at build time, a class added
//     alongside the markup that uses it would be missing outright rather than
//     merely out of date. A hashed URL is never already in the cache, so the
//     two always arrive together.
//
//     Only the unhashed assets are pre-cached; the hashed pair is cached on
//     demand by the first page load, which happens before anything could need
//     them offline.
//   - HTML pages use a network-first strategy: try the server, fall back to the
//     cached last-good copy if offline. Auth-protected pages won't render
//     offline (the server redirects to /login when no cookie is present), but
//     the user at least sees a cached page instead of a browser error.
//   - Anything POST/PUT/DELETE is passed straight through so writes never get
//     silently swallowed by the cache.

// Bumping VERSION drops every previous cache on activate. v3 exists to evict
// the unhashed app.css that v2 pinned, which is what left installed copies
// rendering new markup against an old stylesheet.
const VERSION = 'wodl-v3';
const SHELL_CACHE = `${VERSION}-shell`;
const PAGE_CACHE = `${VERSION}-pages`;

const SHELL_URLS = [
  '/static/icon-192.png',
  '/static/icon-512.png',
  '/static/icon-512-maskable.png',
  '/manifest.webmanifest',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((cache) => cache.addAll(SHELL_URLS))
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((k) => !k.startsWith(VERSION))
          .map((k) => caches.delete(k))
      )
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;

  // Stale-while-revalidate for static assets: answer from cache for speed and
  // offline support, refreshing in the background. Hashed URLs miss the cache
  // on a new build and are fetched outright, so the stylesheet always matches
  // the markup that asked for it.
  if (url.pathname.startsWith('/static/') || url.pathname === '/manifest.webmanifest') {
    event.respondWith(
      caches.open(SHELL_CACHE).then((cache) =>
        cache.match(req).then((hit) => {
          const fetching = fetch(req)
            .then((res) => {
              if (res && res.ok) cache.put(req, res.clone());
              return res;
            })
            .catch(() => hit);
          return hit || fetching;
        })
      )
    );
    return;
  }

  // Network-first for navigations and HTML pages.
  const accept = req.headers.get('accept') || '';
  if (req.mode === 'navigate' || accept.includes('text/html')) {
    event.respondWith(
      fetch(req)
        .then((res) => {
          if (res && res.ok) {
            const copy = res.clone();
            caches.open(PAGE_CACHE).then((cache) => cache.put(req, copy));
          }
          return res;
        })
        .catch(() => caches.match(req).then((hit) => hit || caches.match('/')))
    );
  }
});
