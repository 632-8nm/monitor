// Service worker: offline shell for the monitor dashboard.
// - Navigations: network-first, fall back to the cached shell when offline
// - Static assets: stale-while-revalidate (serve cached, refresh in background)
// - /api/*: never intercepted — monitoring data must always be live
const CACHE = 'monitor-shell-v1';

self.addEventListener('install', (e) => {
	e.waitUntil(
		caches.open(CACHE).then((c) => c.addAll([
			'/',
			'/manifest.json',
			'/static/icons/icon-192.png',
			'/static/icons/icon-512.png',
		])).then(() => self.skipWaiting())
	);
});

self.addEventListener('activate', (e) => {
	e.waitUntil(
		caches.keys().then((keys) =>
			Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
		).then(() => self.clients.claim())
	);
});

self.addEventListener('fetch', (e) => {
	const url = new URL(e.request.url);
	if (e.request.method !== 'GET' || url.origin !== location.origin) return;
	if (url.pathname.startsWith('/api/')) return; // live data only

	if (url.pathname === '/' || url.pathname === '/index.html' || url.pathname === '/admin') {
		// network-first with cache fallback: fresh page when online, shell when offline
		e.respondWith(
			fetch(e.request)
				.then((r) => {
					const copy = r.clone();
					caches.open(CACHE).then((c) => c.put(url.pathname, copy));
					return r;
				})
				.catch(() => caches.match(url.pathname).then((hit) => hit || caches.match('/')))
		);
		return;
	}

	// stale-while-revalidate for css/js/icons
	e.respondWith(
		caches.match(e.request).then((hit) => {
			const refresh = fetch(e.request).then((r) => {
				if (r.ok) {
					const copy = r.clone();
					caches.open(CACHE).then((c) => c.put(e.request, copy));
				}
				return r;
			});
			return hit || refresh;
		})
	);
});
