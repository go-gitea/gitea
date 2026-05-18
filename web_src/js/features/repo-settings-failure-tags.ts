import {createApp} from 'vue';
import RepoActionsFailureTagsSettings from '../components/RepoActionsFailureTagsSettings.vue';

export function initActionRunFailureTagsSettings() {
  const el = document.querySelector('#repo-actions-failure-tags');
  if (!el) return;
  createApp(RepoActionsFailureTagsSettings, {
    apiUrl: el.getAttribute('data-api-url')!,
    locale: {
      name: el.getAttribute('data-locale-name')!,
      color: el.getAttribute('data-locale-color')!,
      description: el.getAttribute('data-locale-description')!,
      add: el.getAttribute('data-locale-add')!,
      edit: el.getAttribute('data-locale-edit')!,
      save: el.getAttribute('data-locale-save')!,
      cancel: el.getAttribute('data-locale-cancel')!,
      delete: el.getAttribute('data-locale-delete')!,
      confirmDelete: el.getAttribute('data-locale-confirm-delete')!,
    },
  }).mount(el);
}
