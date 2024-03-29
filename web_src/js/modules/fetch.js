import {isObject} from '../utils.js';

const {csrfToken} = window.config;

// safe HTTP methods that don't need a csrf token
const safeMethods = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);

// fetch wrapper, use below method name functions and the `data` option to pass in data
// which will automatically set an appropriate headers. For json content, only object
// and array types are currently supported.
export function request(url, {method = 'GET', data, headers = {}, ...other} = {}) {
  let body, contentType;
  if (data instanceof FormData || data instanceof URLSearchParams) {
    body = data;
  } else if (isObject(data) || Array.isArray(data)) {
    contentType = 'application/json';
    body = JSON.stringify(data);
  }

  const headersMerged = new Headers({
    ...(!safeMethods.has(method) && {'x-csrf-token': csrfToken}),
    ...(contentType && {'content-type': contentType}),
  });

  for (const [name, value] of Object.entries(headers)) {
    headersMerged.set(name, value);
  }

  return fetch(url, {
    method,
    headers: headersMerged,
    ...other,
    ...(body && {body}),
  });
}

export const GET = (url, opts) => request(url, {method: 'GET', ...opts});
export const POST = (url, opts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url, opts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url, opts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url, opts) => request(url, {method: 'DELETE', ...opts});
