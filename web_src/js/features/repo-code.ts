import {svg} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {clippie} from 'clippie';
import {toAbsoluteUrl} from '../utils.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';

function changeHash(hash: string) {
  if (window.history.pushState) {
    window.history.pushState(null, null, hash);
  } else {
    window.location.hash = hash;
  }
}

// it selects the code lines defined by range: `L1-L3` (3 lines) or `L2` (singe line)
function selectRange(range: string): Element {
  for (const el of document.querySelectorAll('.code-view tr.active')) el.classList.remove('active');
  const elLineNums = document.querySelectorAll(`.code-view td.lines-num span[data-line-number]`);

  const refInNewIssue = document.querySelector('a.ref-in-new-issue');
  const copyPermalink = document.querySelector('a.copy-line-permalink');
  const viewGitBlame = document.querySelector('a.view_git_blame');

  const updateIssueHref = function (anchor: string) {
    if (!refInNewIssue) return;
    const urlIssueNew = refInNewIssue.getAttribute('data-url-issue-new');
    const urlParamBodyLink = refInNewIssue.getAttribute('data-url-param-body-link');
    const issueContent = `${toAbsoluteUrl(urlParamBodyLink)}#${anchor}`; // the default content for issue body
    refInNewIssue.setAttribute('href', `${urlIssueNew}?body=${encodeURIComponent(issueContent)}`);
  };

  const updateViewGitBlameFragment = function (anchor: string) {
    if (!viewGitBlame) return;
    let href = viewGitBlame.getAttribute('href');
    href = `${href.replace(/#L\d+$|#L\d+-L\d+$/, '')}`;
    if (anchor.length !== 0) {
      href = `${href}#${anchor}`;
    }
    viewGitBlame.setAttribute('href', href);
  };

  const updateCopyPermalinkUrl = function (anchor: string) {
    if (!copyPermalink) return;
    let link = copyPermalink.getAttribute('data-url');
    link = `${link.replace(/#L\d+$|#L\d+-L\d+$/, '')}#${anchor}`;
    copyPermalink.setAttribute('data-url', link);
  };

  const rangeFields = range ? range.split('-') : [];
  const start = rangeFields[0] ?? '';
  if (!start) return null;
  const stop = rangeFields[1] || start;

  // format is i.e. 'L14-L26'
  let startLineNum = parseInt(start.substring(1));
  let stopLineNum = parseInt(stop.substring(1));
  if (startLineNum > stopLineNum) {
    const tmp = startLineNum;
    startLineNum = stopLineNum;
    stopLineNum = tmp;
    range = `${stop}-${start}`;
  }

  const first = elLineNums[startLineNum - 1] ?? null;
  for (let i = startLineNum - 1; i <= stopLineNum - 1 && i < elLineNums.length; i++) {
    elLineNums[i].closest('tr').classList.add('active');
  }
  changeHash(`#${range}`);
  updateIssueHref(range);
  updateViewGitBlameFragment(range);
  updateCopyPermalinkUrl(range);
  return first;
}

function showLineButton() {
  const menu = document.querySelector('.code-line-menu');
  if (!menu) return;

  // remove all other line buttons
  for (const el of document.querySelectorAll('.code-line-button')) {
    el.remove();
  }

  // find active row and add button
  const tr = document.querySelector('.code-view tr.active');
  if (!tr) return;

  const td = tr.querySelector('td.lines-num');
  const btn = document.createElement('button');
  btn.classList.add('code-line-button', 'ui', 'basic', 'button');
  btn.innerHTML = svg('octicon-kebab-horizontal');
  td.prepend(btn);

  // put a copy of the menu back into DOM for the next click
  btn.closest('.code-view').append(menu.cloneNode(true));

  createTippy(btn, {
    theme: 'menu',
    trigger: 'click',
    hideOnClick: true,
    content: menu,
    placement: 'right-start',
    interactive: true,
    onShow: (tippy) => {
      tippy.popper.addEventListener('click', () => {
        tippy.hide();
      }, {once: true});
    },
  });
}

export function initRepoCodeView() {
  if (!document.querySelector('.code-view .lines-num')) return;

  let selRangeStart: string;
  addDelegatedEventListener(document, 'click', '.lines-num span', (el: HTMLElement, e: KeyboardEvent) => {
    if (!selRangeStart || !e.shiftKey) {
      selRangeStart = el.getAttribute('id');
      selectRange(selRangeStart);
    } else {
      const selRangeStop = el.getAttribute('id');
      selectRange(`${selRangeStart}-${selRangeStop}`);
    }
    window.getSelection().removeAllRanges();
    showLineButton();
  });

  const onHashChange = () => {
    if (!window.location.hash) return;
    const range = window.location.hash.substring(1);
    const first = selectRange(range);
    if (first) {
      // set scrollRestoration to 'manual' when there is a hash in url, so that the scroll position will not be remembered after refreshing
      if (window.history.scrollRestoration !== 'manual') window.history.scrollRestoration = 'manual';
      first.scrollIntoView({block: 'start'});
      showLineButton();
    }
  };
  onHashChange();
  window.addEventListener('hashchange', onHashChange);

  addDelegatedEventListener(document, 'click', '.copy-line-permalink', (el) => {
    clippie(toAbsoluteUrl(el.getAttribute('data-url')));
  });
}
