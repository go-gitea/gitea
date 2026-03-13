// bootstrap module must be the first one to be imported, it handles global errors
import './bootstrap.ts';

import '../fomantic/build/fomantic.css';
import '../css/index.css';

// many users expect to use jQuery in their custom scripts (https://docs.gitea.com/administration/customizing-gitea#example-plantuml)
// so load globals (including jQuery) as early as possible
import './globals.ts';

import './modules/user-settings.ts'; // templates also need to use localUserSettings in inline scripts
import {onDomReady} from './utils/dom.ts';

// TODO: There is a bug in htmx, it incorrectly checks "readyState === 'complete'" when the DOM tree is ready and won't trigger DOMContentLoaded
// Then importing the htmx in our onDomReady will make htmx skip its initialization.
// If the bug would be fixed (https://github.com/bigskysoftware/htmx/pull/3365), then we can only import htmx in "onDomReady"
import 'htmx.org';

onDomReady(async () => {
  try {
    await import('./index-domready.ts');
  } catch (e) {
    // When navigating away before the dynamic import completes, a TypeError is thrown.
    // The error message varies across browsers, so we can't check for a specific string.
    if (e instanceof TypeError) {
      console.error('Error loading index-domready:', e);
    } else {
      throw e;
    }
  }
});
