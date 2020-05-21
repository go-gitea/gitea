import {registerRoute} from 'workbox-routing';
import {StaleWhileRevalidate} from 'workbox-strategies';

const cachedDestinations = new Set([
  'manifest',
  'script',
  'style',
  'worker',
]);

registerRoute(
  ({request}) => cachedDestinations.has(request.destination),
  new StaleWhileRevalidate({cacheName: 'static-cache-v2'}),
);
