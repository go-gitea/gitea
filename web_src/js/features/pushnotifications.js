export default async function initPushNotificationsOptIn() {
  if (!window.location.pathname.startsWith('/notifications')) return;
  if (!('serviceWorker' in navigator && 'PushManager' in window && window.config.ServiceWorkerEnabled)) {
    return;
  }

  const button = $('<a class="ui button" href="#">Enable Push Notifications</a>');
  button.on('click', subscribe);
  $('#pushNotificationOptIn').prepend(button);
}

async function subscribe() {
  const canNotify = await hasNotificationPermission();
  if (!canNotify) return false;

  /** @type {ServiceWorkerRegistration} */
  const registration = window.serviceWorkerRegistration;
  const subscriptionResults = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: window.config.WebPushPublicKey
  });
  await createGiteaServerSubscription(subscriptionResults.toJSON());
}

async function createGiteaServerSubscription(subscriptionJSON) {
  try {
    const request = await fetch(`${window.config.AppSubUrl}/api/v1/notifications/subscription`, {
      credentials: 'include',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json; charset=utf-8',
      },
      body: JSON.stringify({
        endpoint: subscriptionJSON.endpoint,
        auth: subscriptionJSON.keys.auth,
        p256dh: subscriptionJSON.keys.p256dh
      })
    });
    if (request.status === 201) return true;
  } catch (error) {
    console.error(error);
  }
  return false;
}

async function hasNotificationPermission() {
  const requestResult = await requestNotificationPermission();
  if (requestResult === 'granted') {
    return true;
  }
  return false;
}

function requestNotificationPermission() {
  return new Promise((resolve, reject) => {
    // This used to be callback-based instead of a Promise. We account for that here:
    const permissionResult = Notification.requestPermission((result) => {
      return resolve(result);
    });

    if (permissionResult) {
      permissionResult.then(resolve, reject);
    }
  });
}
