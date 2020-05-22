const {UseServiceWorker, AppSubUrl, AppVer} = window.config;
const cacheName = 'static-cache-v2';

async function unregister() {
  for (const registration of await navigator.serviceWorker.getRegistrations()) {
    const serviceWorker = registration.active;
    if (!serviceWorker) continue;
    registration.unregister();
  }
}

async function invalidateCache() {
  await caches.delete(cacheName);
}

async function checkCacheValidity() {
  const cacheKey = AppVer;
  const storedCacheKey = localStorage.getItem('staticCacheKey');

  // invalidate cache if it belongs to a different gitea version
  if (cacheKey && storedCacheKey !== cacheKey) {
    invalidateCache();
    localStorage.setItem('staticCacheKey', cacheKey);
  }
}

export default async function initServiceWorker() {
  if (!('serviceWorker' in navigator)) return;

  if (UseServiceWorker) {
    await checkCacheValidity();
    try {
      await navigator.serviceWorker.register(`${AppSubUrl}/serviceworker.js`);
    } catch (err) {
      console.error(err);
      await invalidateCache();
      await unregister();
    }
  } else {
    await invalidateCache();
    await unregister();
  }
}
