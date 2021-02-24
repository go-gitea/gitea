import {registerRoute} from 'workbox-routing';
import {StaleWhileRevalidate} from 'workbox-strategies';

const cacheName = 'static-cache-v2';

// disable workbox debug logging in development, remove when debugging the service worker
self.__WB_DISABLE_DEV_LOGS = true;

// see https://developer.mozilla.org/en-US/docs/Web/API/RequestDestination for possible values
const cachedDestinations = new Set([
  'font',
  'manifest',
  'paintworklet',
  'script',
  'sharedworker',
  'style',
  'worker',
]);

registerRoute(
  ({request}) => cachedDestinations.has(request.destination),
  new StaleWhileRevalidate({cacheName}),
);
