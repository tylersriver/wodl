// Service worker for WODL PWA.
//
// Strategy:
//   - Static shell (stylesheet, htmx, icons, manifest) is pre-cached so an
//     installed app launches and renders correctly even without network. It is
//     served stale-while-revalidate: the cached copy answers immediately and a
//     background fetch refreshes it, so a redeployed app.css lands on the next
//     launch rather than being pinned until the VERSION below changes.
//   - HTML pages use a network-first strategy: try the server, fall back to the
//     cached last-good copy if offline. Auth-protected pages won't render
//     offline (the server redirects to /login when no cookie is present), but
//     the user at least sees a cached page instead of a browser error.
//   - Anything POST/PUT/DELETE is passed straight through so writes never get
//     silently swallowed by the cache.

const VERSION = 'wodl-v2';
const SHELL_CACHE = `${VERSION}-shell`;
const PAGE_CACHE = `${VERSION}-pages`;

const SHELL_URLS = [
  '/static/app.css',
  '/static/htmx.min.js',
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

  // Stale-while-revalidate for the static shell: answer from cache for speed
  // and offline support, but always refresh in the background so a new build's
  // stylesheet is picked up without waiting for a VERSION bump.
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
