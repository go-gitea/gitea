const {UseServiceWorker, AppSubUrl, AppVer} = window.config;
const cachePrefix = 'static-cache-v'; // actual version is set in the service worker script

async function unregister() {
  const registrations = await navigator.serviceWorker.getRegistrations();
  await Promise.all(registrations.map((registration) => {
    return registration.active && registration.unregister();
  }));
}

async function invalidateCache() {
  const cacheKeys = await caches.keys();
  await Promise.all(cacheKeys.map((key) => {
    return key.startsWith(cachePrefix) && caches.delete(key);
  }));
}

async function checkCacheValidity() {
  const cacheKey = AppVer;
  const storedCacheKey = localStorage.getItem('staticCacheKey');

  // invalidate cache if it belongs to a different gitea version
  if (cacheKey && storedCacheKey !== cacheKey) {
    await invalidateCache();
    localStorage.setItem('staticCacheKey', cacheKey);
  }
}

export default async function initServiceWorker() {
  if (!('serviceWorker' in navigator)) return;

  if (UseServiceWorker) {
    try {
      // normally we'd serve the service worker as a static asset from StaticUrlPrefix but
      // the spec strictly requires it to be same-origin so it has to be AppSubUrl to work
      await Promise.all([
        checkCacheValidity(),
        navigator.serviceWorker.register(`${AppSubUrl}/assets/serviceworker.js`),
      ]);
    } catch (err) {
      console.error(err);
      await Promise.all([
        invalidateCache(),
        unregister(),
      ]);
    }
  } else {
    await Promise.all([
      invalidateCache(),
      unregister(),
    ]);
  }
}
