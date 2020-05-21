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
      navigator.serviceWorker.register(`${AppSubUrl}/serviceworker.js`);
    } catch (err) {
      console.error(err);
      await unregister();
    }
  } else {
    await unregister();
  }
}
