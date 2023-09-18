import {isObject} from '../utils.js';

const {csrfToken} = window.config;

// fetch wrapper, use below method name functions and the `data` option to pass in data
// which will automatically set an appropriate content-type header. For json content,
// only object and array types are currently supported.
function request(url, {headers, data, body, ...other} = {}) {
  let contentType;
  if (!body) {
    if (data instanceof FormData) {
      contentType = 'multipart/form-data';
      body = data;
    } else if (data instanceof URLSearchParams) {
      contentType = 'application/x-www-form-urlencoded';
      body = data;
    } else if (isObject(data) || Array.isArray(data)) {
      contentType = 'application/json';
      body = JSON.stringify(data);
    }
  }

  return fetch(url, {
    headers: {
      'x-csrf-token': csrfToken,
      ...(contentType && {'content-type': contentType}),
      ...headers,
    },
    ...(body && {body}),
    ...other,
  });
}

export const GET = (url, opts) => request(url, {method: 'GET', ...opts});
export const POST = (url, opts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url, opts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url, opts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url, opts) => request(url, {method: 'DELETE', ...opts});
