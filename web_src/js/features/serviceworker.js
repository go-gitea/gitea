const {UseServiceWorker, AppSubUrl, AppVer} = window.config;
const cacheName = 'static-cache-v2';

async function unregister() {
  for (const registration of await navigator.serviceWorker.getRegistrations()) {
    const serviceWorker = registration.active;
    if (!serviceWorker) continue;
    registration.unregister();
  }
}

export default async function initServiceWorker() {
  if (!('serviceWorker' in navigator)) return;

  const cacheKey = AppVer;
  const storedCacheKey = localStorage.getItem('serviceWorkerCacheKey');

  // invalidate cache if it belongs to a different gitea version
  if (cacheKey && storedCacheKey !== cacheKey) {
    try {
      await caches.delete(cacheName);
    } catch (_err) {}
  }

  // register or unregister the service worker script
  if (UseServiceWorker) {
    try {
      await navigator.serviceWorker.register(`${AppSubUrl}/serviceworker.js`);
      if (cacheKey) {
        localStorage.setItem('serviceWorkerCacheKey', cacheKey);
      }
    } catch (err) {
      console.error(err);
      await unregister();
    }
  } else {
    await unregister();
  }
}
