// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.ts';
import './webcomponents/index.ts';
import {onDomReady} from './utils/dom.ts';

// TODO: There is a bug in htmx, it incorrectly checks "readyState === 'complete'" when the DOM tree is ready and won't trigger DOMContentLoaded
// Then importing the htmx in our onDomReady will make htmx skip its initialization.
// If the bug would be fixed (https://github.com/bigskysoftware/htmx/pull/3365), then we can only import htmx in "onDomReady"
import 'htmx.org';

onDomReady(async () => {
  // when navigate before the import complete, there will be an error from webpack chunk loader:
  // JavaScript promise rejection: Loading chunk index-domready failed.
  try {
    await import(/* webpackChunkName: "index-domready" */'./index-domready.ts');
  } catch (e) {
    if (e.name === 'ChunkLoadError') {
      console.error('Error loading index-domready:', e);
    } else {
      throw e;
    }
  }
});
