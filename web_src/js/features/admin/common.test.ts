import {initAdminRunnerBulk} from './common.ts';

test('initAdminRunnerBulk updates selected runner IDs', () => {
  document.body.innerHTML = `
    <div class="tw-hidden" data-global-init="initRunnerBulkToolbar">
      <form><input type="hidden" name="ids"></form>
      <button class="runner-bulk-action"><span class="runner-bulk-count"></span></button>
    </div>
    <input type="checkbox" class="runner-bulk-select-all">
    <input type="checkbox" class="runner-bulk-select" data-runner-id="1">
    <input type="checkbox" class="runner-bulk-select" data-runner-id="2">
  `;

  const toolbar = document.querySelector<HTMLElement>('[data-global-init="initRunnerBulkToolbar"]')!;
  const runnerIds = toolbar.querySelector<HTMLInputElement>('input[name="ids"]')!;
  const runners = document.querySelectorAll<HTMLInputElement>('.runner-bulk-select');
  const selectAll = document.querySelector<HTMLInputElement>('.runner-bulk-select-all')!;
  initAdminRunnerBulk(toolbar);

  runners[0].checked = true;
  runners[0].dispatchEvent(new Event('change'));
  expect(runnerIds.value).toBe('1');

  selectAll.checked = true;
  selectAll.dispatchEvent(new Event('change'));
  expect(runnerIds.value).toBe('1,2');

  runners[0].checked = false;
  runners[0].dispatchEvent(new Event('change'));
  expect(runnerIds.value).toBe('2');
});
