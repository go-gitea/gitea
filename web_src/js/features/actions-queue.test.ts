import {updateActionQueueList} from './actions-queue.ts';

function queueHTML(disabled: boolean, job: string): string {
  return `<div id="actions-queue-list" data-queue-refresh-link="?refresh=1&job=${job}">
    <div id="actions-queue-filter"><div class="ui dropdown ${disabled ? 'disabled' : ''}">
      <input type="text"><div class="menu"><a>${job}</a></div>
    </div></div>
    <div class="queue-job">${job}</div>
  </div>`;
}

test('queue refresh replaces disabled filters when new work arrives', () => {
  document.body.innerHTML = queueHTML(true, 'empty');
  const el = document.querySelector<HTMLElement>('#actions-queue-list')!;
  const incoming = document.createElement('div');
  incoming.innerHTML = queueHTML(false, 'new-repo');

  updateActionQueueList(el, incoming.firstElementChild!.cloneNode(true) as HTMLElement);

  expect(el.querySelector('.dropdown.disabled')).toBeNull();
  expect(el.querySelector('.menu a')!.textContent).toBe('new-repo');
  expect(el.querySelector('.queue-job')!.textContent).toBe('new-repo');
});

test('queue refresh keeps rows live while deferring filter updates', () => {
  document.body.innerHTML = queueHTML(false, 'old-repo');
  const el = document.querySelector<HTMLElement>('#actions-queue-list')!;
  const dropdown = el.querySelector('.dropdown')!;
  const input = el.querySelector('input')!;
  const incoming = document.createElement('div');
  incoming.innerHTML = queueHTML(false, 'new-repo');
  dropdown.classList.add('active');
  input.value = 'search';

  updateActionQueueList(el, incoming.firstElementChild!.cloneNode(true) as HTMLElement);
  expect(el.querySelector('.queue-job')!.textContent).toBe('new-repo');
  expect(el.querySelector('.menu a')!.textContent).toBe('old-repo');
  expect(input.value).toBe('search');
  expect(dropdown.classList.contains('active')).toBe(true);

  dropdown.classList.remove('active');
  input.focus();
  updateActionQueueList(el, incoming.firstElementChild!.cloneNode(true) as HTMLElement);
  expect(document.activeElement).toBe(input);
  expect(el.querySelector('.queue-job')!.textContent).toBe('new-repo');
  expect(el.querySelector('.menu a')!.textContent).toBe('old-repo');

  input.blur();
  updateActionQueueList(el, incoming.firstElementChild!.cloneNode(true) as HTMLElement);
  expect(el.querySelector('.queue-job')!.textContent).toBe('new-repo');
  expect(el.querySelector('.menu a')!.textContent).toBe('new-repo');
  expect(el.getAttribute('data-queue-refresh-link')).toContain('job=new-repo');
});
