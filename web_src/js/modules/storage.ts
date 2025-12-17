/** Get a setting from localStorage */
export function getLocalStorageSetting(key: string) {
  return localStorage?.getItem(key);
}

/** Set a setting in localStorage */
export function setLocalStorageSetting(key: string, value: string) {
  return localStorage?.setItem(key, value);
}

/** Add a listener to the 'storage' event for given setting key. This event only fires in non-current tabs. */
export function addLocalStorageChangeListener(key: string, listener: (e: StorageEvent) => void) {
  const fn = (e: StorageEvent) => {
    if (e.storageArea === localStorage && e.key === key) {
      listener(e);
    }
  };
  window.addEventListener('storage', fn);
  return fn; // for unregister
}
