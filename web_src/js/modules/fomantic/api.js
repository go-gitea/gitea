import $ from 'jquery';
import {GET} from '../fetch.js';
import {isObject} from '../../utils.js';

// action: "search"
// cache: true
// debug: false
// on: false
// onAbort: function onAbort(response)
// onError: function error()
// onFailure: function onFailure()
// onResponse: function onResponse(response)
// onSuccess: function onSuccess(response)
// url: "/user/search?active=1&q={query}"
// urlData: Object { query: "si" }

export function initFomanticApi() {
  // stand-in for removed api module
  // https://github.com/fomantic/Fomantic-UI/blob/2.8.7/src/definitions/modules/dropdown.js
  // https://github.com/fomantic/Fomantic-UI/blob/2.8.7/src/definitions/behaviors/api.js
  $.fn.api = function (arg0) {
    console.info(arg0);

    if (arg0 === 'is loading') return this._loading;
    if (arg0 === 'abort') {
      this._ac?.abort();
      return;
    }

    if (isObject(arg0)) {
      let {url, urlData, onSuccess, onError, onAbort} = arg0;
      if (url.includes('{query}') && urlData?.query) {
        url = url.replace('{query}', urlData.query);
      }
      this._data = {url, onSuccess, onError, onAbort};
    } else if (arg0 === 'query') {
      (async () => {
        const {url, onSuccess, onError, onAbort} = this._data;

        try {
          this._loading = true;
          this._ac = new AbortController();
          const res = await GET(url, {signal: this.ac.signal});
          if (!res.ok) {
            onError?.();
          }

          if (res?.headers?.['content-type']?.startsWith('application/json')) {
            onSuccess?.(await res.json());
          } else {
            onSuccess?.(await res.text());
          }
        } catch (err) {
          this._loading = false;
          if (err.name === 'AbortError') {
            onAbort?.();
          } else {
            onError?.();
          }
        }
      })();
    }

    return this;
  };
}
