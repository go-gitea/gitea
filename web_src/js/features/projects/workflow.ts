import {createApp} from 'vue';
import ProjectWorkflow from '../../components/projects/ProjectWorkflow.vue';

export async function initProjectWorkflow() {
  const workflowDiv = document.querySelector('#project-workflows');
  if (!workflowDiv) return;

  createApp(ProjectWorkflow, {
    projectLink: workflowDiv.getAttribute('data-project-link'),
    eventID: workflowDiv.getAttribute('data-event-id'),
  }).mount(workflowDiv);
}
