import './polyfills.ts';
import './relative-time.ts';
import './origin-url.ts';
import './overflow-menu.ts';
import {isDarkTheme} from '../utils.ts';

function initPageThemeDarkLight() {
  // Set page's theme color preference as early as possible, to avoid flicker of wrong theme color during page load.
  const sync = () => document.documentElement.setAttribute('data-gitea-theme-dark', String(isDarkTheme()));
  sync();
  // Track system theme changes in case Gitea is using "auto" theme.
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', sync);
}

initPageThemeDarkLight();
