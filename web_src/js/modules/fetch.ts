import {isObject} from '../utils.ts';
import type {RequestOpts} from '../types.ts';

const {csrfToken} = window.config;

// safe HTTP methods that don't need a csrf token
const safeMethods = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);

// fetch wrapper, use below method name functions and the `data` option to pass in data
// which will automatically set an appropriate headers. For json content, only object
// and array types are currently supported.
export function request(url: string, {method = 'GET', data, headers = {}, ...other}: RequestOpts = {}): Promise<Response> {
  let body: string | FormData | URLSearchParams;
  let contentType: string;
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

export const GET = (url: string, opts?: RequestOpts) => request(url, {method: 'GET', ...opts});
export const POST = (url: string, opts?: RequestOpts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url: string, opts?: RequestOpts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url: string, opts?: RequestOpts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url: string, opts?: RequestOpts) => request(url, {method: 'DELETE', ...opts});
