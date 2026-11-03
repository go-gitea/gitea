import {createApp} from 'vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import type {ProjectWorkflowLocale} from '../components/ProjectWorkflowStore.ts';

async function mountProjectWorkflow(el: HTMLElement) {
  const {default: ProjectWorkflow} = await import('../components/ProjectWorkflow.vue');
  createApp(ProjectWorkflow, {
    el,
    projectLink: el.getAttribute('data-project-link')!,
    workflowId: Number(el.getAttribute('data-workflow-id')),
    workflowEvent: el.getAttribute('data-workflow-event')!,
    canWrite: el.getAttribute('data-can-write') === 'true',
    locale: JSON.parse(el.getAttribute('data-locale')!) as ProjectWorkflowLocale,
  }).mount(el);
}

export function initProjectWorkflow() {
  registerGlobalInitFunc('initProjectWorkflow', mountProjectWorkflow);
}
