import {createApp} from 'vue';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {createElementFromHTMLOrNull, activePageTimerRefresh} from '../utils/dom.ts';
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

export function applyMergeBoxRefresh(el: Element, html: string): boolean {
  const newEl = createElementFromHTMLOrNull(html);
  if (!newEl) return false; // unexpected empty response, keep the old element instead of destroying it
  const scrollTop = el.querySelector<HTMLElement>('.commit-status-list')?.scrollTop;
  el.replaceWith(newEl); // don't morph, do full replacement to make sure data-global-init and Vue components are re-initialized
  if (scrollTop) newEl.querySelector<HTMLElement>('.commit-status-list')?.scrollTo({top: scrollTop, behavior: 'instant'});
  return true;
}

function initRepoPullMergeBoxRefresh(el: Element) {
  // The merge box has complex buttons & form, if the user has interacted with any element, don't refresh.
  // Otherwise, the user won't be able to merge or schedule a merge (auto-merge) when the PR status is not ready.
  let interacted = false;
  const interactionEvents = ['focusin', 'mousedown', 'click', 'keydown', 'input'];
  for (const event of interactionEvents) {
    el.addEventListener(event, () => { interacted = true }, {capture: true});
  }

  activePageTimerRefresh({
    once: true, // on successful refresh, the data-global-init will re-initialize the element
    interval: () => interacted ? 0 : Number(el.getAttribute('data-pull-merge-box-reloading-interval')),
    async callback() {
      if (interacted) return;
      const pullLink = el.getAttribute('data-pull-link')!;
      const resp = await GET(`${pullLink}/merge_box`);
      if (!resp.ok) return;
      applyMergeBoxRefresh(el, await resp.text());
    },
  });
}

export function initRepoPullMergeBox(el: HTMLElement) {
  registerGlobalEventFunc('click', 'onCommitStatusChecksToggle', onCommitStatusChecksToggle);
  initRepoPullRequestMergeForm(el);
  initRepoPullMergeBoxRefresh(el);
}
