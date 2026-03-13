// bootstrap module must be the first one to be imported, it handles global errors
import {initGlobalErrorHandler} from './bootstrap.ts';

// many users expect to use jQuery in their custom scripts (https://docs.gitea.com/administration/customizing-gitea#example-plantuml)
// so load globals (including jQuery) as early as possible
import './globals.ts';

import './webcomponents/index.ts';
import './modules/user-settings.ts'; // templates also need to use localUserSettings in inline scripts

// TODO: There is a bug in htmx, it incorrectly checks "readyState === 'complete'" when the DOM tree is ready and won't trigger DOMContentLoaded
// Then importing the htmx in our onDomReady will make htmx skip its initialization.
// If the bug would be fixed (https://github.com/bigskysoftware/htmx/pull/3365), then we can only import htmx in "onDomReady"
import 'htmx.org';

initGlobalErrorHandler();
