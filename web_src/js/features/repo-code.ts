import {svg} from '../svg.ts';
import {createTippy} from '../modules/tippy.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';
import {registerAsyncHashChangeListener} from '../modules/hashchange.ts';

export function selectLineRows(first: HTMLTableRowElement, last: HTMLTableRowElement) {
  const rows = [first];
  while (rows[rows.length - 1] !== last) rows.push(rows[rows.length - 1].nextElementSibling as HTMLTableRowElement);
  return rows;
}

function updateLineMenu(menu: Element, anchor: string, rows: HTMLTableRowElement[]) {
  const copyLines = menu.querySelector('button.copy-lines')!;
  if (rows.length > 1) copyLines.textContent = copyLines.getAttribute('data-text-lines');
  copyLines.setAttribute('data-clipboard-text', rows.map((row) => row.querySelector('.code-inner')!.textContent.replace(/[\r\n]+$/, '')).join('\n'));

  const copyPermalink = menu.querySelector('button.copy-line-permalink')!;
  const permalink = new URL(copyPermalink.getAttribute('data-url')!, window.location.origin);
  permalink.hash = anchor;
  copyPermalink.setAttribute('data-clipboard-text', permalink.href);

  const viewGitBlame = menu.querySelector('a.view_git_blame');
  if (viewGitBlame) viewGitBlame.setAttribute('href', `${viewGitBlame.getAttribute('href')}#${anchor}`);

  const refInNewIssue = menu.querySelector('a.ref-in-new-issue');
  if (refInNewIssue) {
    const issueContent = `${refInNewIssue.getAttribute('data-url-param-body-link')}#${anchor}`;
    refInNewIssue.setAttribute('href', `${refInNewIssue.getAttribute('data-url-issue-new')}?body=${encodeURIComponent(issueContent)}`);
  }
}

function showLineButton(first: Element, anchor: string, rows: HTMLTableRowElement[]) {
  const menu = document.querySelector('.code-line-menu')?.cloneNode(true) as Element | undefined;
  if (!menu) return;

  const btn = document.createElement('button');
  btn.type = 'button';
  btn.classList.add('code-line-button', 'ui', 'basic', 'button');
  btn.innerHTML = svg('octicon-kebab-horizontal');
  first.parentElement!.prepend(btn);

  updateLineMenu(menu, anchor, rows);
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

function selectRange(hash: string) {
  for (const row of document.querySelectorAll('.code-view tr.active')) row.classList.remove('active');
  for (const btn of document.querySelectorAll('.code-line-button')) {
    btn._tippy.destroy();
    btn.remove();
  }

  const match = /^#L([1-9]\d*)(?:-L([1-9]\d*))?$/.exec(hash);
  if (!match) return;
  const firstLine = Number(match[1]), lastLine = Number(match[2] ?? match[1]);
  const start = Math.min(firstLine, lastLine), end = Math.max(firstLine, lastLine);
  const lineNumbers = document.querySelectorAll('.code-view td.lines-num span[data-line-number]');
  const first = lineNumbers[start - 1];
  if (!first) return;
  const last = lineNumbers[Math.min(end, lineNumbers.length) - 1];
  const rows = selectLineRows(first.closest('tr')!, last.closest('tr')!);
  for (const row of rows) row.classList.add('active');

  const anchor = start === end ? `L${start}` : `L${start}-L${end}`;
  showLineButton(first, anchor, rows);
  return {anchor, first};
}

export function initRepoCodeView() {
  // When viewing a file or blame, there is always a ".file-view" element,
  // but the ".code-view" class is only present when viewing the "code" of a file; it is not present when viewing a PDF file.
  // Since the ".file-view" will be dynamically reloaded when navigating via the left file tree (eg: view a PDF file, then view a source code file, etc.)
  // the "code-view" related event listeners should always be added when the current page contains ".file-view" element.
  if (!document.querySelector('.repo-view-container .file-view')) return;

  let selRangeStart: string | undefined;
  addDelegatedEventListener<HTMLElement, MouseEvent>(document, 'click', '.code-view .lines-num span[data-line-number]', (el, e) => {
    if (!selRangeStart || !e.shiftKey) selRangeStart = el.id;
    const selection = selectRange(`#${selRangeStart}-${el.id}`);
    if (selection) {
      const hash = `#${selection.anchor}`;
      if (window.location.hash !== hash) window.history.pushState(null, '', hash);
    }
    window.getSelection()!.removeAllRanges();
  });

  registerAsyncHashChangeListener(() => {
    const selection = selectRange(window.location.hash);
    selRangeStart = selection?.first.id;
    if (selection) {
      window.history.scrollRestoration = 'manual';
      selection.first.scrollIntoView({block: 'start'});
    }
  });
}
