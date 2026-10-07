import {createApp} from 'vue';
import type {ProjectWorkflowLocale} from '../components/ProjectWorkflowStore.ts';

export async function initProjectWorkflow() {
  const el = document.querySelector('#project-workflows');
  if (!el) return;

  const {default: ProjectWorkflow} = await import('../components/ProjectWorkflow.vue');
  createApp(ProjectWorkflow, {
    projectLink: el.getAttribute('data-project-link')!,
    workflowKey: el.getAttribute('data-workflow-key')!,
    canWrite: el.getAttribute('data-can-write') === 'true',
    locale: JSON.parse(el.getAttribute('data-locale')!) as ProjectWorkflowLocale,
  }).mount(el);
}
