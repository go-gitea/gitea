import {createApp} from 'vue';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {registerGlobalEventFunc} from '../modules/observer.ts';

export function initRepoPullRequestUpdate(el: HTMLElement) {
  const elDropdown = el.querySelector(':scope > .ui.dropdown');
  if (!elDropdown) return;
  const elButton = el.querySelector<HTMLButtonElement>(':scope > button')!;

  fomanticQuery(elDropdown).dropdown({
    onChange(_text: string, _value: string, $choice: any) {
      const choiceEl = $choice[0];
      elButton.textContent = choiceEl.textContent;
      elButton.setAttribute('data-url', choiceEl.getAttribute('data-update-url'));
    },
  });
}

function onCommitStatusChecksToggle(btn: HTMLElement) {
  const panel = btn.closest('.commit-status-toggle')!.parentElement!;
  const list = panel.querySelector<HTMLElement>('.commit-status-list')!;
  list.style.maxHeight = list.style.maxHeight ? '' : '0px'; // toggle
  btn.textContent = btn.getAttribute(list.style.maxHeight ? 'data-show-all' : 'data-hide-all');
}

async function initRepoPullRequestMergeForm(box: HTMLElement) {
  const el = box.querySelector('#pull-request-merge-form');
  if (!el) return;

  const data = JSON.parse(el.getAttribute('data-merge-form-props')!);
  const {default: PullRequestMergeForm} = await import('../components/PullRequestMergeForm.vue');
  const view = createApp(PullRequestMergeForm, {mergeFormProps: data});
  view.mount(el); // TODO: can unmount when reloaded?
}

export function initRepoPullMergeBox(el: HTMLElement) {
  registerGlobalEventFunc('click', 'onCommitStatusChecksToggle', onCommitStatusChecksToggle);
  initRepoPullRequestMergeForm(el);

  const reloadingIntervalValue = el.getAttribute('data-pull-merge-box-reloading-interval');
  if (!reloadingIntervalValue) return;

  const reloadingInterval = parseInt(reloadingIntervalValue);
  const pullLink = el.getAttribute('data-pull-link');
  let timerId: number | null;

  let reloadMergeBox: () => Promise<void>;
  const stopReloading = () => {
    if (!timerId) return;
    clearTimeout(timerId);
    timerId = null;
  };
  const startReloading = () => {
    if (timerId) return;
    setTimeout(reloadMergeBox, reloadingInterval);
  };
  const onVisibilityChange = () => {
    if (document.hidden) {
      stopReloading();
    } else {
      startReloading();
    }
  };
  reloadMergeBox = async () => {
    const resp = await GET(`${pullLink}/merge_box`);
    stopReloading();
    if (!resp.ok) {
      startReloading();
      return;
    }
    document.removeEventListener('visibilitychange', onVisibilityChange);
    const newElem = createElementFromHTML(await resp.text());
    el.replaceWith(newElem);
  };

  document.addEventListener('visibilitychange', onVisibilityChange);
  startReloading();
}
