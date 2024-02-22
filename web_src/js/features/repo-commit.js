import {createTippy} from '../modules/tippy.js';
import {toggleElem} from '../utils/dom.js';
import {parseDom} from '../utils.js';
import {POST} from '../modules/fetch.js';

export function initRepoEllipsisButton() {
  for (const button of document.querySelectorAll('.js-toggle-commit-body')) {
    button.addEventListener('click', function (e) {
      e.preventDefault();
      const expanded = this.getAttribute('aria-expanded') === 'true';
      toggleElem(this.parentElement.querySelector('.commit-body'));
      this.setAttribute('aria-expanded', String(!expanded));
    });
  }
}

export async function initRepoCommitLastCommitLoader() {
  const entryMap = {};

  const entries = Array.from(document.querySelectorAll('table#repo-files-table tr.notready'), (el) => {
    const entryName = el.getAttribute('data-entryname');
    entryMap[entryName] = el;
    return entryName;
  });

  if (entries.length === 0) {
    return;
  }

  const lastCommitLoaderURL = document.querySelector('table#repo-files-table').getAttribute('data-last-commit-loader-url');

  if (entries.length > 200) {
    // For more than 200 entries, replace the entire table
    const response = await POST(lastCommitLoaderURL);
    const data = await response.text();
    document.querySelector('table#repo-files-table').outerHTML = data;
    return;
  }

  // For fewer entries, update individual rows
  const response = await POST(lastCommitLoaderURL, {data: {'f': entries}});
  const data = await response.text();
  const doc = parseDom(data, 'text/html');
  for (const row of doc.querySelectorAll('tr')) {
    if (row.className === 'commit-list') {
      document.querySelector('table#repo-files-table .commit-list')?.replaceWith(row);
      continue;
    }
    // there are other <tr> rows in response (eg: <tr class="has-parent">)
    // at the moment only the "data-entryname" rows should be processed
    const entryName = row.getAttribute('data-entryname');
    if (entryName) {
      entryMap[entryName]?.replaceWith(row);
    }
  }
}

export function initCommitStatuses() {
  for (const element of document.querySelectorAll('[data-tippy="commit-statuses"]')) {
    const top = document.querySelector('.repository.file.list') || document.querySelector('.repository.diff');

    createTippy(element, {
      content: element.nextElementSibling,
      placement: top ? 'top-start' : 'bottom-start',
      interactive: true,
      role: 'dialog',
      theme: 'box-with-header',
    });
  }
}
