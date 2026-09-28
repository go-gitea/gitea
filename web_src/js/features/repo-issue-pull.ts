import {createApp, type App, type Component} from 'vue';
import {GET} from '../modules/fetch.ts';
import {createTippy} from '../modules/tippy.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {addDelegatedEventListener, createElementFromHTML, activePageTimerRefresh} from '../utils/dom.ts';

let chosenUpdateUrl = '';
let chosenMergeStyle = '';
let mergeFormComponent: Component | undefined; // cached so a refresh remounts the form before the next paint
let mergeFormApp: App | undefined;

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
  if (data.mergeStyles.some((style: {name: string}) => style.name === chosenMergeStyle)) data.defaultMergeStyle = chosenMergeStyle;
  mergeFormComponent ??= (await import('../components/PullRequestMergeForm.vue')).default;
  mergeFormApp = createApp(mergeFormComponent, {mergeFormProps: data, onMergeStyleChange: (style: string) => { chosenMergeStyle = style }});
  mergeFormApp.mount(el);
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
      const newEl = createElementFromHTML(respText);
      const openDetails = new Map([...el.querySelectorAll<HTMLDetailsElement>('[data-kind]')].map((details) => [details.getAttribute('data-kind'), details.open]));
      for (const details of newEl.querySelectorAll<HTMLDetailsElement>('[data-kind]')) {
        const open = openDetails.get(details.getAttribute('data-kind'));
        if (open !== undefined) details.open = open;
      }
      const scrollTop = el.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTop;
      mergeFormApp?.unmount();
      mergeFormApp = undefined;
      el.replaceWith(newEl); // don't morph, do full replacement to make sure data-global-init and Vue components are re-initialized
      if (scrollTop) newEl.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTo({top: scrollTop, behavior: 'instant'});
    },
  });
}

export function initRepoPullCloneUrl(el: HTMLElement) {
  const url = el.getAttribute(`data-clone-${localUserSettings.getString('repo-clone-protocol')}`);
  if (url) el.textContent = url;
}

export function initRepoPullMergeBox(el: HTMLElement) {
  initRepoPullRequestMergeForm(el);
  initRepoPullMergeBoxRefresh(el);
}
