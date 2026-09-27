import {createApp, type Component} from 'vue';
import {GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {addDelegatedEventListener, createElementFromHTML, activePageTimerRefresh} from '../utils/dom.ts';

let chosenUpdateUrl = ''; // survives the merge box refresh
let mergeFormComponent: Component | undefined; // cached so a refresh remounts the form before the next paint

export function initRepoPullRequestUpdate(el: HTMLElement) {
  const [elButton, elTrigger] = el.querySelectorAll<HTMLButtonElement>(':scope > button');
  const menu = el.nextElementSibling!;
  const menuTippy = createTippy(elTrigger, {
    content: menu,
    getReferenceClientRect: () => el.getBoundingClientRect(),
    theme: 'menu',
    arrow: false,
    maxWidth: 400,
    placement: 'bottom-end',
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
  });
  const choose = (choice: Element) => {
    chosenUpdateUrl = choice.getAttribute('data-update-url')!;
    for (const item of menu.querySelectorAll('.item')) item.setAttribute('aria-checked', String(item === choice));
    elButton.textContent = choice.getAttribute('data-update-text');
    elButton.setAttribute('data-url', chosenUpdateUrl);
  };
  addDelegatedEventListener(menu, 'click', '.item', (choice) => {
    choose(choice);
    menuTippy.hide();
  });
  const chosen = [...menu.querySelectorAll('.item')].find((item) => item.getAttribute('data-update-url') === chosenUpdateUrl);
  if (chosen) choose(chosen);
}

async function initRepoPullRequestMergeForm(box: HTMLElement) {
  const el = box.querySelector('#pull-request-merge-form');
  if (!el) return;

  const data = JSON.parse(el.getAttribute('data-merge-form-props')!);
  mergeFormComponent ??= (await import('../components/PullRequestMergeForm.vue')).default;
  const view = createApp(mergeFormComponent, {mergeFormProps: data});
  view.mount(el); // TODO: can unmount when reloaded?
}

function initRepoPullMergeBoxRefresh(el: Element) {
  // like GitHub, skip the refresh while a menu or the merge form is open or rules are being bypassed
  const isBusy = () => Boolean(el.querySelector('[aria-expanded="true"], #pull-request-merge-form form, #merge-bypass-rules:checked'));
  activePageTimerRefresh({
    interval: () => el.isConnected ? Number(el.getAttribute('data-pull-merge-box-reloading-interval')) : 0,
    async callback() {
      if (isBusy()) return;
      const pullLink = el.getAttribute('data-pull-link')!;
      const resp = await GET(`${pullLink}/merge_box`);
      if (!resp.ok) return;
      const respText = (await resp.text()).trim();
      if (isBusy()) return;
      if (!respText) {
        el.remove(); // merge box might not exist if the PR has changed (e.g.: merged and the head branch has been deleted)
        return;
      }
      const newEl = createElementFromHTML(respText);
      const checks = el.querySelector<HTMLDetailsElement>('.merge-box-checks');
      const newChecks = newEl.querySelector<HTMLDetailsElement>('.merge-box-checks');
      if (checks && newChecks) newChecks.open = checks.open;
      const scrollTop = el.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTop;
      el.replaceWith(newEl); // don't morph, do full replacement to make sure data-global-init and Vue components are re-initialized
      if (scrollTop) newEl.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTo({top: scrollTop, behavior: 'instant'});
    },
  });
}

export function initRepoPullMergeBox(el: HTMLElement) {
  initRepoPullRequestMergeForm(el);
  initRepoPullMergeBoxRefresh(el);
}
