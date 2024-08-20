import {createApp} from 'vue';

export async function initScopedAccessTokenCategories() {
  const el = document.querySelector('#scoped-access-token-selector');
  if (!el) return;

  const {default: ScopedAccessTokenSelector} = await import(/* webpackChunkName: "scoped-access-token-selector" */'../components/ScopedAccessTokenSelector.vue');
  try {
    const View = createApp(ScopedAccessTokenSelector, {
      isAdmin: JSON.parse(el.getAttribute('data-is-admin')),
      noAccessLabel: el.getAttribute('data-no-access-label'),
      readLabel: el.getAttribute('data-read-label'),
      writeLabel: el.getAttribute('data-write-label'),
    });
    View.mount(el);
  } catch (err) {
    console.error('ScopedAccessTokenSelector failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
