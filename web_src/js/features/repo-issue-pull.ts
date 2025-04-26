import {createApp} from 'vue';
import PullRequestMergeForm from '../components/PullRequestMergeForm.vue';
import {GET, POST} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {createElementFromHTML} from '../utils/dom.ts';

function initRepoPullRequestUpdate(el: HTMLElement) {
  const prUpdateButtonContainer = el.querySelector('#update-pr-branch-with-base');
  if (!prUpdateButtonContainer) return;

  const prUpdateButton = prUpdateButtonContainer.querySelector<HTMLButtonElement>(':scope > button');
  const prUpdateDropdown = prUpdateButtonContainer.querySelector(':scope > .ui.dropdown');
  prUpdateButton.addEventListener('click', async function (e) {
    e.preventDefault();
    const redirect = this.getAttribute('data-redirect');
    this.classList.add('is-loading');
    let response: Response;
    try {
      response = await POST(this.getAttribute('data-do'));
    } catch (error) {
      console.error(error);
    } finally {
      this.classList.remove('is-loading');
    }
    let data: Record<string, any>;
    try {
      data = await response?.json(); // the response is probably not a JSON
    } catch (error) {
      console.error(error);
    }
    if (data?.redirect) {
      window.location.href = data.redirect;
    } else if (redirect) {
      window.location.href = redirect;
    } else {
      window.location.reload();
    }
  });

  fomanticQuery(prUpdateDropdown).dropdown({
    onChange(_text: string, _value: string, $choice: any) {
      const choiceEl = $choice[0];
      const url = choiceEl.getAttribute('data-do');
      if (url) {
        const buttonText = prUpdateButton.querySelector('.button-text');
        if (buttonText) {
          buttonText.textContent = choiceEl.textContent;
        }
        prUpdateButton.setAttribute('data-do', url);
      }
    },
  });
}

function initRepoPullRequestCommitStatus(el: HTMLElement) {
  for (const btn of el.querySelectorAll('.commit-status-hide-checks')) {
    const panel = btn.closest('.commit-status-panel');
    const list = panel.querySelector<HTMLElement>('.commit-status-list');
    btn.addEventListener('click', () => {
      list.style.maxHeight = list.style.maxHeight ? '' : '0px'; // toggle
      btn.textContent = btn.getAttribute(list.style.maxHeight ? 'data-show-all' : 'data-hide-all');
    });
  }
}

function initRepoPullRequestMergeForm(box: HTMLElement) {
  const el = box.querySelector('#pull-request-merge-form');
  if (!el) return;

  const view = createApp(PullRequestMergeForm);
  view.mount(el);
}

function executeScripts(elem: HTMLElement) {
  for (const oldScript of elem.querySelectorAll('script')) {
    // TODO: that's the only way to load the data for the merge form. In the future
    //  we need to completely decouple the page data and embedded script
    // eslint-disable-next-line github/no-dynamic-script-tag
    const newScript = document.createElement('script');
    for (const attr of oldScript.attributes) {
      if (attr.name === 'type' && attr.value === 'module') continue;
      newScript.setAttribute(attr.name, attr.value);
    }
    newScript.text = oldScript.text;
    document.body.append(newScript);
  }
}

export function initRepoPullMergeBox(el: HTMLElement) {
  initRepoPullRequestCommitStatus(el);
  initRepoPullRequestUpdate(el);
  initRepoPullRequestMergeForm(el);

  const reloadingIntervalValue = el.getAttribute('data-pull-merge-box-reloading-interval');
  if (!reloadingIntervalValue) return;

  const reloadingInterval = parseInt(reloadingIntervalValue);
  const pullLink = el.getAttribute('data-pull-link');
  let timerId: number;

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
    executeScripts(newElem);
    el.replaceWith(newElem);
  };

  document.addEventListener('visibilitychange', onVisibilityChange);
  startReloading();
}
