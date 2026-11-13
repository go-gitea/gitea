import {createApp, type App, type Component} from 'vue';
import {GET} from '../modules/fetch.ts';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {addDelegatedEventListener, createElementFromHTML, activePageTimerRefresh} from '../utils/dom.ts';

const reloadingIntervalAttr = 'data-pull-merge-box-reloading-interval';

// owns the merge box element across refreshes, so the user's choices survive them
class PullMergeBox {
  private readonly el: HTMLElement;
  private chosenUpdateUrl = '';
  private chosenMergeStyle = '';
  private mergeFormComponent?: Component;
  private mergeFormApp?: App;

  constructor(el: HTMLElement) {
    this.el = el;
  }

  async init() {
    await this.initContent();
    activePageTimerRefresh({
      interval: () => this.el.isConnected ? Number(this.el.getAttribute(reloadingIntervalAttr)) : 0,
      callback: () => this.refresh(),
    });
  }

  private async initContent() {
    this.initUpdateStyleMenu();
    await this.initMergeForm();
  }

  private initUpdateStyleMenu() {
    const elButtons = this.el.querySelector('[data-pull-update-buttons]');
    if (!elButtons) return;
    const [elButton, elTrigger] = elButtons.querySelectorAll<HTMLButtonElement>(':scope > button');
    const menu = this.el.querySelector('[data-pull-update-menu]')!;
    const menuTippy = createTippy(elTrigger, {
      content: menu,
      getReferenceClientRect: () => elButtons.getBoundingClientRect(),
      theme: 'menu',
      arrow: false,
      maxWidth: 400,
      placement: 'bottom-end',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
    });
    const choose = (choice: Element) => {
      this.chosenUpdateUrl = choice.getAttribute('data-update-url')!;
      for (const item of menu.querySelectorAll('.item')) item.setAttribute('aria-checked', String(item === choice));
      elButton.textContent = choice.getAttribute('data-update-text');
      elButton.setAttribute('data-url', this.chosenUpdateUrl);
    };
    addDelegatedEventListener(menu, 'click', '.item', (choice) => {
      choose(choice);
      menuTippy.hide();
    });
    const chosen = [...menu.querySelectorAll('.item')].find((item) => item.getAttribute('data-update-url') === this.chosenUpdateUrl);
    if (chosen) choose(chosen);
  }

  private async initMergeForm() {
    const elForm = this.el.querySelector('[data-merge-form-props]');
    if (!elForm) return;

    const data = JSON.parse(elForm.getAttribute('data-merge-form-props')!);
    if (data.mergeStyles.some((style: {name: string}) => style.name === this.chosenMergeStyle)) data.defaultMergeStyle = this.chosenMergeStyle;
    this.mergeFormComponent ??= (await import('../components/PullRequestMergeForm.vue')).default; // cached so a refresh remounts the form before the next paint
    if (!elForm.isConnected) return;
    this.mergeFormApp = createApp(this.mergeFormComponent, {mergeFormProps: data, onMergeStyleChange: (style: string) => { this.chosenMergeStyle = style }});
    this.mergeFormApp.mount(elForm);
  }

  private isBusy() {
    // keep focused controls, open menus and dialogs, forms, bypass choices and pending requests
    return this.el.contains(document.activeElement) || Boolean(document.querySelector('#pull-merge-cmd-modal[open]') ||
      this.el.querySelector('[aria-expanded="true"], [data-fetch-is-loading], [data-pull-merge-busy]'));
  }

  private async refresh() {
    if (this.isBusy()) return;
    const pullLink = this.el.getAttribute('data-pull-link')!;
    const resp = await GET(`${pullLink}/merge_box`);
    if (!resp.ok) return;
    const respText = (await resp.text()).trim();
    if (this.isBusy()) return;
    const newEl = createElementFromHTML(respText);
    const openDetails = new Map([...this.el.querySelectorAll<HTMLDetailsElement>('[data-kind]')].map((details) => [details.getAttribute('data-kind'), details.open]));
    for (const details of newEl.querySelectorAll<HTMLDetailsElement>('[data-kind]')) {
      const open = openDetails.get(details.getAttribute('data-kind'));
      if (open !== undefined) details.open = open;
    }
    const scrollTop = this.el.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTop;
    const oldElements = this.el.querySelectorAll('*');
    this.mergeFormApp?.unmount();
    this.mergeFormApp = undefined;
    for (const oldElement of oldElements) getAttachedTippyInstance(oldElement)?.destroy();
    this.el.replaceChildren(...newEl.childNodes); // don't morph, so data-global-init and Vue components are re-initialized
    this.el.setAttribute(reloadingIntervalAttr, newEl.getAttribute(reloadingIntervalAttr) ?? '0');
    await this.initContent();
    if (scrollTop) this.el.querySelector<HTMLElement>('.merge-box-checks-body')?.scrollTo({top: scrollTop, behavior: 'instant'});
  }
}

export function initRepoPullCloneUrl(el: HTMLElement) {
  const url = el.getAttribute(`data-clone-${localUserSettings.getString('repo-clone-protocol')}`);
  if (url) el.textContent = url;
}

export function initRepoPullMergeBox(el: HTMLElement) {
  return new PullMergeBox(el).init();
}
