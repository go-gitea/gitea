import {createApp} from 'vue';

export async function initScopedAccessTokenCategories() {
  const el = document.querySelector('#scoped-access-token-selector');
  if (!el) return;

  const {default: ScopedAccessTokenForm} = await import(/* webpackChunkName: "scoped-access-token-form" */'../components/ScopedAccessTokenForm.vue');
  try {
    const View = createApp(ScopedAccessTokenForm, {
      isAdmin: JSON.parse(el.getAttribute('data-is-admin')),
      noAccessLabel: el.getAttribute('data-no-access-label'),
      readLabel: el.getAttribute('data-read-label'),
      writeLabel: el.getAttribute('data-write-label'),
    });
    View.mount(el);
  } catch (err) {
    console.error('ScopedAccessTokenForm failed to load', err);
    el.textContent = el.getAttribute('data-locale-component-failed-to-load');
  }
}
