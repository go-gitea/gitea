import {toggleElemClass} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoGraphGit() {
  const graphContainer = document.querySelector<HTMLElement>('#git-graph-container');
  if (!graphContainer) return;

  const elColorMonochrome = document.querySelector<HTMLElement>('#flow-color-monochrome');
  const elColorColored = document.querySelector<HTMLElement>('#flow-color-colored');
  const toggleColorMode = (mode: 'monochrome' | 'colored') => {
    toggleElemClass(graphContainer, 'monochrome', mode === 'monochrome');
    toggleElemClass(graphContainer, 'colored', mode === 'colored');

    toggleElemClass(elColorMonochrome, 'active', mode === 'monochrome');
    toggleElemClass(elColorColored, 'active', mode === 'colored');

    const params = new URLSearchParams(window.location.search);
    params.set('mode', mode);
    window.history.replaceState(null, '', `?${params.toString()}`);
    for (const link of document.querySelectorAll('#git-graph-body .pagination a')) {
      const href = link.getAttribute('href');
      if (!href) continue;
      const url = new URL(href, window.location.href);
      const params = url.searchParams;
      params.set('mode', mode);
      url.search = `?${params.toString()}`;
      link.setAttribute('href', url.href);
    }
  };
  elColorMonochrome.addEventListener('click', () => toggleColorMode('monochrome'));
  elColorColored.addEventListener('click', () => toggleColorMode('colored'));

  const elGraphBody = document.querySelector<HTMLElement>('#git-graph-body');
  const url = new URL(window.location.href);
  const params = url.searchParams;
  const loadGitGraph = async () => {
    const queryString = params.toString();
    const ajaxUrl = new URL(url);
    ajaxUrl.searchParams.set('div-only', 'true');
    window.history.replaceState(null, '', queryString ? `?${queryString}` : window.location.pathname);

    elGraphBody.classList.add('is-loading');
    try {
      const resp = await GET(ajaxUrl.toString());
      elGraphBody.innerHTML = await resp.text();
    } finally {
      elGraphBody.classList.remove('is-loading');
    }
  };

  const dropdownSelected = params.getAll('branch');
  if (params.has('hide-pr-refs') && params.get('hide-pr-refs') === 'true') {
    dropdownSelected.splice(0, 0, '...flow-hide-pr-refs');
  }

  const $dropdown = fomanticQuery('#flow-select-refs-dropdown');
  $dropdown.dropdown({clearable: true});
  $dropdown.dropdown('set selected', dropdownSelected);
  // must add the callback after setting the selected items, otherwise each "selected" item will trigger the callback
  $dropdown.dropdown('setting', {
    onRemove(toRemove: string) {
      if (toRemove === '...flow-hide-pr-refs') {
        params.delete('hide-pr-refs');
      } else {
        const branches = params.getAll('branch');
        params.delete('branch');
        for (const branch of branches) {
          if (branch !== toRemove) {
            params.append('branch', branch);
          }
        }
      }
      loadGitGraph();
    },
    onAdd(toAdd: string) {
      if (toAdd === '...flow-hide-pr-refs') {
        params.set('hide-pr-refs', 'true');
      } else {
        params.append('branch', toAdd);
      }
      loadGitGraph();
    },
  });
}
