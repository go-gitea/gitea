const {UseServiceWorker, AppSubUrl} = window.config;

async function unregister() {
  for (const registration of await navigator.serviceWorker.getRegistrations()) {
    const serviceWorker = registration.active;
    if (!serviceWorker) continue;
    registration.unregister();
  }
}

export default async function initServiceWorker() {
  if (!('serviceWorker' in navigator)) return;

  if (UseServiceWorker) {
    try {
      await navigator.serviceWorker.register(`${AppSubUrl}/serviceworker.js`);
    } catch (err) {
      await unregister();
      throw err;
    }
  } else {
    await unregister();
  }
}
