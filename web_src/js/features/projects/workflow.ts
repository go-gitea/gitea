import {createApp} from 'vue';
import ProjectWorkflow from '../../components/projects/ProjectWorkflow.vue';
import {readWorkflowLocale} from '../../components/projects/workflowLocale.ts';

export async function initProjectWorkflow() {
  const workflowDiv = document.querySelector('#project-workflows');
  if (!workflowDiv) return;

  const View = createApp(ProjectWorkflow, {
    projectLink: workflowDiv.getAttribute('data-project-link')!,
    eventId: workflowDiv.getAttribute('data-event-id') ?? '',
    canWriteProjects: workflowDiv.getAttribute('data-can-write-projects') === 'true',
    locale: readWorkflowLocale(workflowDiv),
  });
  View.mount(workflowDiv);
}
