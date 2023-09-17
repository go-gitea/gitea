import {isObject} from '../utils.js';

const {csrfToken} = window.config;

// fetch wrapper, use below method name functions and the `data` option to pass in data
// which will automatically set an appropriate content-type header. For json content,
// only object and array types are currently supported.
export function request(url, {headers = {}, data, body, ...other} = {}) {
  let contentType;
  if (!body) {
    if (data instanceof FormData) {
      body = data;
    } else if (data instanceof URLSearchParams) {
      body = data;
    } else if (isObject(data) || Array.isArray(data)) {
      contentType = 'application/json';
      body = JSON.stringify(data);
    }
  }

  const headersMerged = new Headers({
    'x-csrf-token': csrfToken,
    ...(contentType && {'content-type': contentType}),
  });

  // not using spread syntax to avoid undesirable value merging
  for (const [name, value] of Object.entries(headers)) {
    headersMerged.set(name, value);
  }

  return fetch(url, {
    headers: headersMerged,
    ...(body && {body}),
    ...other,
  });
}

export const GET = (url, opts) => request(url, {method: 'GET', ...opts});
export const POST = (url, opts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url, opts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url, opts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url, opts) => request(url, {method: 'DELETE', ...opts});
