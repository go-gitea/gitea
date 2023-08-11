export function initRepoActionList() {
  const disableWorkflowBtn = document.getElementById('disable-workflow-btn');
  if (disableWorkflowBtn) {
    disableWorkflowBtn.addEventListener('click', () => {
      document.getElementById('disable-workflow-form').submit();
    });
  }
}
