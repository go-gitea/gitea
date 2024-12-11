export function initRepoMilestone() {
  const page = document.querySelector('.repository.new.milestone');
  if (!page) return;

  const deadline = page.querySelector<HTMLInputElement>('form input[name=deadline]');
  document.querySelector('#milestone-clear-deadline').addEventListener('click', () => {
    deadline.value = '';
  });
}
