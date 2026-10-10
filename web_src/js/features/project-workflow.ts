import {createApp} from 'vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import type {ProjectWorkflowLocale} from '../components/ProjectWorkflowStore.ts';

async function mountProjectWorkflow(el: HTMLElement) {
  const {default: ProjectWorkflow} = await import('../components/ProjectWorkflow.vue');
  createApp(ProjectWorkflow, {
    el,
    projectLink: el.getAttribute('data-project-link')!,
    workflowKey: el.getAttribute('data-workflow-key')!,
    canWrite: el.getAttribute('data-can-write') === 'true',
    locale: JSON.parse(el.getAttribute('data-locale')!) as ProjectWorkflowLocale,
  }).mount(el);
}

export function initProjectWorkflow() {
  registerGlobalInitFunc('initProjectWorkflow', mountProjectWorkflow);
}
