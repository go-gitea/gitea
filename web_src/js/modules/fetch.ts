import {isObject} from '../utils.ts';
import type {RequestOpts} from '../types.ts';
import isNetworkError from 'is-network-error';

// fetch wrapper, use below method name functions and the `data` option to pass in data
// which will automatically set an appropriate headers. For JSON content, only object
// and array types are currently supported.
export function request(url: string, {method = 'GET', data, headers = {}, ...other}: RequestOpts = {}): Promise<Response> {
  let body: string | FormData | URLSearchParams | undefined;
  let contentType: string | undefined;
  if (data instanceof FormData || data instanceof URLSearchParams) {
    body = data;
  } else if (isObject(data) || Array.isArray(data)) {
    contentType = 'application/json';
    body = JSON.stringify(data);
  }

  headers = new Headers(headers);
  if (!headers.has('content-type') && contentType) {
    headers.set('content-type', contentType);
  }
  return fetch(url, { // eslint-disable-line no-restricted-globals -- this is the wrapper the rule points to
    method,
    headers,
    ...other,
    ...(body && {body}),
  });
}

export const GET = (url: string, opts?: RequestOpts) => request(url, {method: 'GET', ...opts});
export const POST = (url: string, opts?: RequestOpts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url: string, opts?: RequestOpts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url: string, opts?: RequestOpts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url: string, opts?: RequestOpts) => request(url, {method: 'DELETE', ...opts});

class ExclusiveResponse {
  private readonly promiseResp: Promise<Response>;
  private readonly exclusiveFetch: ExclusiveFetch;
  private readonly abortController: AbortController;
  private onResponseErrorFn: ((details: {message: string, response?: Response}) => void) | null = null;

  constructor(exclusiveFetch: ExclusiveFetch, abortController: AbortController, promiseResp: Promise<Response>) {
    this.exclusiveFetch = exclusiveFetch;
    this.abortController = abortController;
    this.promiseResp = promiseResp;
  }

  private isAborted() {
    return this.abortController.signal.aborted || this.exclusiveFetch.currentController !== this.abortController;
  }

  private async fetchGeneric<T, R>(readResp: (resp: Response) => Promise<T>, callback: (ret: T) => R | Promise<R>) : Promise<R | undefined> {
    try {
      const resp = await this.promiseResp;
      if (this.isAborted()) return undefined;
      if (!resp.ok) {
        this.onResponseErrorFn?.({message: `http: ${resp.status} ${resp.statusText}`, response: resp});
        return undefined;
      }
      const ret = await readResp(resp);
      if (this.isAborted()) return undefined;
      return await callback(ret);
    } catch (err) {
      // avoid network error while unloading page, and ignore "abort" error
      if (isNetworkError(err) || this.isAborted()) return undefined;
      // let caller handle callback function's exceptions
      throw err;
    } finally {
      if (this.exclusiveFetch.currentController === this.abortController) {
        this.exclusiveFetch.currentController = null;
      }
    }
  }

  // onResponseError allows the caller to set a callback function that will be called when the response is not ok (e.g., 4xx or 5xx status codes).
  // The callback is only called when the request is not-ok and not aborted (the request is the last one).
  onResponseError(fn: (details: {message: string, response?: Response}) => void) {
    this.onResponseErrorFn = fn;
  }

  // fetchText reads the response as text and passes it to the provided callback function.
  // The callback is only called when the request is successful and not aborted (the request is the last one).
  async fetchText<R>(fn: (text: string) => R | Promise<R>): Promise<R | undefined> {
    return this.fetchGeneric((resp) => resp.text(), fn);
  }

  async fetchJson<T, R>(fn: (text: T) => R | Promise<R>): Promise<R | undefined> {
    return this.fetchGeneric((resp) => resp.json(), fn);
  }
}

// ExclusiveFetch is used for fetching data where only the latest request is relevant.
// If a new request is made before the previous one completes, the previous request will be aborted and discarded.
// This class is composed with "fetch(request)" and "AbortController" to provide a convenient way to handle exclusive requests:
// * Use AbortController to abort previous requests when a new one is made.
// * Handle network exceptions (caused by abort)
// * Ignore outdated/stale responses
// * Handle network error codes
export class ExclusiveFetch {
  public currentController: AbortController | null = null;

  request(url: string, {method = 'GET', data, headers = {}, ...other}: RequestOpts = {}): ExclusiveResponse {
    if (this.currentController) this.currentController.abort();
    const controller = new AbortController();
    this.currentController = controller;
    return new ExclusiveResponse(this, controller, request(url, {method, data, headers, ...other, signal: controller.signal}));
  }

  isInFlight(): boolean {
    return this.currentController !== null;
  }
}
