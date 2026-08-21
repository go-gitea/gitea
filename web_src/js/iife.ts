// This file is the entry point for the code which should block the page rendering, it is compiled by our "iife" vite plugin

// bootstrap module must be the first one to be imported, it handles global errors
import './bootstrap.ts';

// many users expect to use jQuery in their custom scripts (https://docs.gitea.com/administration/customizing-gitea#example-plantuml)
// so load globals (including jQuery) as early as possible
import './globals.ts';

import './webcomponents/index.ts';
import './modules/user-settings.ts'; // templates also need to use localUserSettings in inline scripts
