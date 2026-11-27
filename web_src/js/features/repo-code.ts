import {initDropdown} from '../modules/dropdown.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';
import {registerAsyncHashChangeListener} from '../modules/hashchange.ts';

function selectRange(codeView: HTMLElement, range: string): Element | null {
  const menu = codeView.querySelector<HTMLElement>('.code-line-menu');
  if (!menu) return null;

  const dropdown = initDropdown(menu);
  dropdown.hide();
  for (const row of codeView.querySelectorAll('tr.active')) row.classList.remove('active');

  const [, start, stop = start] = /^L(\d+)(?:-L(\d+))?$/i.exec(range) ?? [];
  let startLine = Number(start);
  let stopLine = Number(stop);
  if (startLine > stopLine) {
    [startLine, stopLine] = [stopLine, startLine];
    range = `L${stop}-L${start}`;
  }

  const lines = codeView.querySelectorAll('.lines-num span[data-line-number]');
  const first = lines[startLine - 1];
  const parent = first ? first.parentElement! : codeView;
  if (menu.parentElement !== parent) parent.prepend(menu);
  if (!first) return null;

  for (let i = startLine - 1; i < stopLine && i < lines.length; i++) {
    lines[i].closest('tr')!.classList.add('active');
  }
  const hash = `#${range}`;
  if (window.history.pushState) {
    window.history.pushState(null, '', hash);
  } else {
    window.location.hash = hash;
  }

  const refInNewIssue = dropdown.popup.querySelector<HTMLAnchorElement>('.ref-in-new-issue');
  if (refInNewIssue) {
    const issueContent = `${refInNewIssue.getAttribute('data-url-param-body-link')!}#${range}`;
    refInNewIssue.href = `${refInNewIssue.getAttribute('data-url-issue-new')!}?body=${encodeURIComponent(issueContent)}`;
  }
  const viewGitBlame = dropdown.popup.querySelector<HTMLAnchorElement>('.view_git_blame');
  if (viewGitBlame) viewGitBlame.href = `${viewGitBlame.getAttribute('href')!.split('#')[0]}#${range}`;

  const copyPermalink = dropdown.popup.querySelector('.copy-line-permalink')!;
  copyPermalink.setAttribute('data-clipboard-text', `${window.location.origin}${copyPermalink.getAttribute('data-url')!}#${range}`);
  return first;
}

export function initRepoCodeView() {
  const repoView = document.querySelector('.repo-view-container');
  if (!repoView) return;

  const rangeStarts = new WeakMap<Element, string>();
  addDelegatedEventListener(repoView, 'click', '.code-view .lines-num span[data-line-number]', (line: HTMLElement, e: MouseEvent) => {
    const codeView = line.closest<HTMLElement>('.code-view')!;
    const start = e.shiftKey ? rangeStarts.get(codeView) : undefined;
    if (!start) rangeStarts.set(codeView, line.id);
    selectRange(codeView, start ? `${start}-${line.id}` : line.id);
    window.getSelection()!.removeAllRanges();
  });

  registerAsyncHashChangeListener(() => {
    if (!window.location.hash) return;
    const codeView = repoView.querySelector<HTMLElement>('.code-view');
    if (!codeView) return;
    const first = selectRange(codeView, window.location.hash.slice(1));
    if (!first) return;
    window.history.scrollRestoration = 'manual'; // so a refresh scrolls to the hash line instead of the remembered position
    first.scrollIntoView({block: 'start'});
  });
}
