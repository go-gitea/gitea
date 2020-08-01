async function unregister() {
  if (!('serviceWorker' in navigator)) return;
  const registrations = await navigator.serviceWorker.getRegistrations();
  await Promise.all(registrations.map((registration) => {
    return registration.active && registration.unregister();
  }));
}

async function invalidateCache() {
  if (!caches || !caches.keys) return;
  const cacheKeys = await caches.keys();
  await Promise.all(cacheKeys.map((key) => {
    return key.startsWith('static-cache-v') && caches.delete(key);
  }));
}

export default async function initServiceWorker() {
  // we once had a service worker, if it's present, remove it and wipe its cache
  await Promise.all([
    invalidateCache(),
    unregister(),
  ]);
}
