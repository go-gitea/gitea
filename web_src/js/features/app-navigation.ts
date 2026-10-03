import {localUserSettings} from '../modules/user-settings.ts';

export function initAppNavigation() {
  const sidebar = document.querySelector<HTMLElement>('#app-sidebar');
  if (!sidebar) return;
  const content = document.querySelector<HTMLElement>('#app-content')!;
  const toggles = document.querySelectorAll<HTMLButtonElement>('.app-nav-toggle');
  const mobile = window.matchMedia('(max-width: 999.98px)');
  let expanded = !mobile.matches && !localUserSettings.getBoolean(`${window.config.appSubUrl}:navigation-collapsed`);
  const update = () => {
    document.body.classList.toggle('app-nav-open', expanded);
    document.body.classList.toggle('app-nav-collapsed', !expanded);
    sidebar.inert = mobile.matches && !expanded;
    content.inert = mobile.matches && expanded;
    for (const toggle of toggles) toggle.setAttribute('aria-expanded', String(expanded));
  };
  for (const link of sidebar.querySelectorAll<HTMLAnchorElement>('.sidebar-item')) {
    const label = link.querySelector('.sidebar-item-text')!.textContent.trim();
    const count = link.querySelector('.sidebar-item-count')?.textContent.trim();
    const description = count ? `${label} (${count})` : label;
    link.setAttribute('data-tooltip-content', description);
    link.setAttribute('aria-label', description);
    if (link.classList.contains('active')) link.setAttribute('aria-current', 'page');
  }
  for (const toggle of toggles) {
    toggle.addEventListener('click', () => {
      expanded = !expanded;
      if (!mobile.matches) localUserSettings.setBoolean(`${window.config.appSubUrl}:navigation-collapsed`, !expanded);
      update();
      if (toggle.hasAttribute('data-sidebar-search-toggle')) sidebar.querySelector<HTMLInputElement>('input[type=search]')!.focus();
      else if (mobile.matches) {
        if (expanded) sidebar.querySelector<HTMLAnchorElement>('a')!.focus();
        else document.querySelector<HTMLButtonElement>('.app-nav-toolbar .app-nav-toggle')!.focus();
      }
    });
  }
  document.addEventListener('keydown', (event) => {
    if (!mobile.matches || !expanded) return;
    if (event.key === 'Escape') {
      expanded = false;
      update();
      document.querySelector<HTMLButtonElement>('.app-nav-toolbar .app-nav-toggle')!.focus();
    }
    if (event.key !== 'Tab') return;
    const controls = Array.from(sidebar.querySelectorAll<HTMLElement>('a[href], button, input, [tabindex="0"]')).filter((el) => el.getClientRects().length && !el.matches(':disabled'));
    const first = controls[0];
    const last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });
  mobile.addEventListener('change', () => {
    expanded = !mobile.matches && !localUserSettings.getBoolean(`${window.config.appSubUrl}:navigation-collapsed`);
    update();
  });
  update();
}
