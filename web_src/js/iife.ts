// This file is the entry point for the code which should block the page rendering, it is compiled by our "iife" vite plugin

// bootstrap module must be the first one to be imported, it handles global config and errors
import './bootstrap.ts';

import './webcomponents/index.ts';
import './modules/user-settings.ts'; // templates also need to use localUserSettings in inline scripts
