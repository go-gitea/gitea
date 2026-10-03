import {page} from 'vitest/browser';
import '../../css/index.css';
import {initAppNavigation} from './app-navigation.ts';
import {localUserSettings} from '../modules/user-settings.ts';

test('desktop rail stays interactive and mobile drawer restores keyboard focus', () => {
  let isMobile = false;
  const media = new EventTarget() as MediaQueryList;
  Object.defineProperty(media, 'matches', {get: () => isMobile});
  vi.spyOn(window, 'matchMedia').mockReturnValue(media);
  localUserSettings.setBoolean(':navigation-collapsed', false);
  document.body.innerHTML = `
    <div class="app-nav-toolbar"><button class="app-nav-toggle">Menu</button></div>
    <div id="app-sidebar">
      <a href="/">Home</a>
      <button class="app-nav-toggle">Collapse</button>
      <a class="sidebar-item active" href="/issues"><span class="sidebar-item-text">Issues</span></a>
    </div>
    <main id="app-content"></main>`;
  const sidebar = document.querySelector<HTMLElement>('#app-sidebar')!;
  const content = document.querySelector<HTMLElement>('#app-content')!;
  const toggle = sidebar.querySelector<HTMLButtonElement>('button')!;
  const opener = document.querySelector<HTMLButtonElement>('.app-nav-toolbar button')!;
  initAppNavigation();
  toggle.click();
  expect(toggle.getAttribute('aria-expanded')).toBe('false');
  expect(sidebar.inert).toBe(false);
  expect(content.inert).toBe(false);
  expect(localUserSettings.getBoolean(':navigation-collapsed')).toBe(true);
  expect(sidebar.querySelector('.active')!.getAttribute('aria-current')).toBe('page');

  isMobile = true;
  media.dispatchEvent(new Event('change'));
  expect(sidebar.inert).toBe(true);
  opener.click();
  expect(content.inert).toBe(true);
  expect(document.activeElement).toBe(sidebar.querySelector('a'));
  const last = sidebar.querySelector<HTMLAnchorElement>('.sidebar-item')!;
  last.focus();
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', cancelable: true}));
  expect(document.activeElement).toBe(sidebar.querySelector('a'));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}));
  expect(sidebar.inert).toBe(true);
  expect(content.inert).toBe(false);
  expect(document.activeElement).toBe(opener);
  localUserSettings.setBoolean(':navigation-collapsed', false);
  vi.restoreAllMocks();
});

test('rail keeps account controls visible and footer stays on one line', async () => {
  await page.viewport(1280, 800);
  document.body.className = 'app-nav-collapsed';
  document.body.innerHTML = `
    <style>* { box-sizing: border-box; } body { margin: 0; }</style>
    <div class="full height">
      <div id="app-sidebar">
        <nav class="sidebar-inner">
          <a class="sidebar-brand" href="/"><img width="26" height="26" alt="Logo"></a>
          <button class="sidebar-collapse">Expand</button>
          <div class="sidebar-main">${'<a class="sidebar-item" href="/"><span class="sidebar-item-text">Repository</span>+</a>'.repeat(40)}</div>
          <div class="sidebar-context-area"><div class="ui dropdown sidebar-context"><span class="text"><img width="24" height="24" alt="Context"><span class="sidebar-context-name">Context</span></span></div></div>
          <div class="sidebar-foot"><div class="sidebar-foot-row"><div class="ui dropdown sidebar-user"><img width="24" height="24" alt="Account"><span class="sidebar-user-name">Account</span></div><div class="sidebar-icons"><div class="ui dropdown item">+<div class="menu visible"><a class="item">Create repository</a></div></div></div></div></div>
        </nav>
      </div>
      <div id="app-content">
        <footer class="page-footer">
          <div class="left-links"><a>Powered by Gitea</a><span>${'Development build information '.repeat(20)}</span></div>
          <div class="right-links"><a>Theme</a><a>Language</a><a>Licenses</a><a>API</a></div>
        </footer>
      </div>
    </div>`;
  const sidebar = document.querySelector<HTMLElement>('#app-sidebar')!;
  const popup = sidebar.querySelector<HTMLElement>('.sidebar-icons .menu')!;
  popup.style.display = 'block';
  expect(popup.getBoundingClientRect().left).toBeGreaterThanOrEqual(sidebar.getBoundingClientRect().right);
  expect(sidebar.querySelector<HTMLElement>('.sidebar-context-name')!.getBoundingClientRect().width).toBe(0);
  const foot = sidebar.querySelector<HTMLElement>('.sidebar-foot')!;
  const main = sidebar.querySelector<HTMLElement>('.sidebar-main')!;
  expect(Math.round(sidebar.getBoundingClientRect().width)).toBe(64);
  expect(foot.getBoundingClientRect().bottom).toBeLessThanOrEqual(window.innerHeight);
  expect(main.scrollHeight).toBeGreaterThan(main.clientHeight);
  expect(main.scrollWidth).toBe(main.clientWidth);
  const left = document.querySelector<HTMLElement>('.page-footer .left-links')!;
  const right = document.querySelector<HTMLElement>('.page-footer .right-links')!;
  expect(left.getBoundingClientRect().top).toBe(right.getBoundingClientRect().top);
  expect(document.documentElement.scrollWidth).toBe(window.innerWidth);
});
