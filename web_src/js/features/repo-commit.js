import {createTippy} from '../modules/tippy.js';
import {toggleElem} from '../utils/dom.js';
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

const parser = new DOMParser();

export async function initRepoCommitLastCommitLoader() {
  const entryMap = {};

  const entries = Array.from(document.querySelectorAll('table#repo-files-table tr.notready'), (v) => {
    entryMap[v.getAttribute('data-entryname')] = v;
    return v.getAttribute('data-entryname');
  });

  if (entries.length === 0) {
    return;
  }

  const lastCommitLoaderURL = document.querySelector('table#repo-files-table').getAttribute('data-last-commit-loader-url');

  if (entries.length > 200) {
    // For more than 200 entries, replace the entire table
    const response = await POST(lastCommitLoaderURL);
    const data = await response.text();
    const table = document.querySelector('table#repo-files-table');
    const parent = table.parentNode;
    const wrapper = document.createElement('div');
    wrapper.innerHTML = data;
    const newTable = wrapper.querySelector('table');
    if (newTable) {
      parent.replaceChild(newTable, table);
    }
    return;
  }

  // For fewer entries, update individual rows
  const response = POST(lastCommitLoaderURL, {data: {'f': entries}});
  const data = await response.text();
  const doc = parser.parseFromString(data, 'text/html');
  for (const row of doc.querySelectorAll('tr')) {
    if (row.className === 'commit-list') {
      document.querySelector('table#repo-files-table .commit-list')?.replaceWith(row);
      return;
    }

    const entryName = row.getAttribute('data-entryname');
    if (entryName) {
      entryMap[entryName].replaceWith(row);
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
