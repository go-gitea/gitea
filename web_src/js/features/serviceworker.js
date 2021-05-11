import {joinPaths} from '../utils.js';

const {UseServiceWorker, AppSubUrl, AssetUrlPrefix, AppVer} = window.config;
const cachePrefix = 'static-cache-v'; // actual version is set in the service worker script
const workerAssetPath = joinPaths(AssetUrlPrefix, 'serviceworker.js');

async function unregisterAll() {
  for (const registration of await navigator.serviceWorker.getRegistrations()) {
    if (registration.active) await registration.unregister();
  }
}

async function unregisterOtherWorkers() {
  for (const registration of await navigator.serviceWorker.getRegistrations()) {
    const scriptURL = registration.active?.scriptURL || '';
    if (!scriptURL.endsWith(workerAssetPath)) await registration.unregister();
  }
}

async function invalidateCache() {
  for (const key of await caches.keys()) {
    if (key.startsWith(cachePrefix)) caches.delete(key);
  }
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
    // unregister all service workers where scriptURL does not match the current one
    await unregisterOtherWorkers();
    try {
      // normally we'd serve the service worker as a static asset from AssetUrlPrefix but
      // the spec strictly requires it to be same-origin so it has to be AppSubUrl to work
      await checkCacheValidity();
      await navigator.serviceWorker.register(joinPaths(AppSubUrl, workerAssetPath));
    } catch (err) {
      console.error(err);
      await invalidateCache();
      await unregisterAll();
    }
  } else {
    await invalidateCache();
    await unregisterAll();
  }
}
