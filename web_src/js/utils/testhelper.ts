import {onTestFinished} from 'vitest';

/** Record and block navigations, as a real browser forbids stubbing "window.location" */
export function captureNavigations() {
  const navigations: Array<{url: string, type: NavigationType}> = [];
  const onNavigate = (e: NavigateEvent) => {
    navigations.push({url: e.destination.url, type: e.navigationType});
    e.preventDefault();
  };
  window.navigation.addEventListener('navigate', onNavigate);
  onTestFinished(() => window.navigation.removeEventListener('navigate', onNavigate));
  return navigations;
}

export function normalizeTestHtml(s: string) {
  const lines = s.replace(/>\s+</g, '>\n<').trim().split('\n');
  for (let i = 0; i < lines.length; i++) {
    lines[i] = lines[i].trim();
  }
  return lines.join('\n');
}
