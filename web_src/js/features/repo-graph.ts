import {hideElem, showElem, type DOMEvent} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoGraphGit() {
  const graphContainer = document.querySelector<HTMLElement>('#git-graph-container');
  if (!graphContainer) return;

  document.querySelector('#flow-color-monochrome')?.addEventListener('click', () => {
    document.querySelector('#flow-color-monochrome').classList.add('active');
    document.querySelector('#flow-color-colored')?.classList.remove('active');
    graphContainer.classList.remove('colored');
    graphContainer.classList.add('monochrome');
    const params = new URLSearchParams(window.location.search);
    params.set('mode', 'monochrome');
    const queryString = params.toString();
    if (queryString) {
      window.history.replaceState({}, '', `?${queryString}`);
    } else {
      window.history.replaceState({}, '', window.location.pathname);
    }
    for (const link of document.querySelectorAll('.pagination a')) {
      const href = link.getAttribute('href');
      if (!href) continue;
      const url = new URL(href, window.location.href);
      const params = url.searchParams;
      params.set('mode', 'monochrome');
      url.search = `?${params.toString()}`;
      link.setAttribute('href', url.href);
    }
  });

  document.querySelector('#flow-color-colored')?.addEventListener('click', () => {
    document.querySelector('#flow-color-colored').classList.add('active');
    document.querySelector('#flow-color-monochrome')?.classList.remove('active');
    graphContainer.classList.add('colored');
    graphContainer.classList.remove('monochrome');
    for (const link of document.querySelectorAll('.pagination a')) {
      const href = link.getAttribute('href');
      if (!href) continue;
      const url = new URL(href, window.location.href);
      const params = url.searchParams;
      params.delete('mode');
      url.search = `?${params.toString()}`;
      link.setAttribute('href', url.href);
    }
    const params = new URLSearchParams(window.location.search);
    params.delete('mode');
    const queryString = params.toString();
    if (queryString) {
      window.history.replaceState({}, '', `?${queryString}`);
    } else {
      window.history.replaceState({}, '', window.location.pathname);
    }
  });
  const url = new URL(window.location.href);
  const params = url.searchParams;
  const updateGraph = () => {
    const queryString = params.toString();
    const ajaxUrl = new URL(url);
    ajaxUrl.searchParams.set('div-only', 'true');
    window.history.replaceState({}, '', queryString ? `?${queryString}` : window.location.pathname);
    document.querySelector('#pagination').innerHTML = '';
    hideElem('#rel-container');
    hideElem('#rev-container');
    showElem('#loading-indicator');
    (async () => {
      const response = await GET(String(ajaxUrl));
      const html = await response.text();
      const div = document.createElement('div');
      div.innerHTML = html;
      document.querySelector('#pagination').innerHTML = div.querySelector('#pagination').innerHTML;
      document.querySelector('#rel-container').innerHTML = div.querySelector('#rel-container').innerHTML;
      document.querySelector('#rev-container').innerHTML = div.querySelector('#rev-container').innerHTML;
      hideElem('#loading-indicator');
      showElem('#rel-container');
      showElem('#rev-container');
    })();
  };
  const dropdownSelected = params.getAll('branch');
  if (params.has('hide-pr-refs') && params.get('hide-pr-refs') === 'true') {
    dropdownSelected.splice(0, 0, '...flow-hide-pr-refs');
  }

  const flowSelectRefsDropdown = document.querySelector('#flow-select-refs-dropdown');
  fomanticQuery(flowSelectRefsDropdown).dropdown('set selected', dropdownSelected);
  fomanticQuery(flowSelectRefsDropdown).dropdown({
    clearable: true,
    fullTextSeach: 'exact',
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
      updateGraph();
    },
    onAdd(toAdd: string) {
      if (toAdd === '...flow-hide-pr-refs') {
        params.set('hide-pr-refs', 'true');
      } else {
        params.append('branch', toAdd);
      }
      updateGraph();
    },
  });

  graphContainer.addEventListener('mouseenter', (e: DOMEvent<MouseEvent>) => {
    if (e.target.matches('#rev-list li')) {
      const flow = e.target.getAttribute('data-flow');
      if (flow === '0') return;
      document.querySelector(`#flow-${flow}`)?.classList.add('highlight');
      e.target.classList.add('hover');
      for (const item of document.querySelectorAll(`#rev-list li[data-flow='${flow}']`)) {
        item.classList.add('highlight');
      }
    } else if (e.target.matches('#rel-container .flow-group')) {
      e.target.classList.add('highlight');
      const flow = e.target.getAttribute('data-flow');
      for (const item of document.querySelectorAll(`#rev-list li[data-flow='${flow}']`)) {
        item.classList.add('highlight');
      }
    } else if (e.target.matches('#rel-container .flow-commit')) {
      const rev = e.target.getAttribute('data-rev');
      document.querySelector(`#rev-list li#commit-${rev}`)?.classList.add('hover');
    }
  });

  graphContainer.addEventListener('mouseleave', (e: DOMEvent<MouseEvent>) => {
    if (e.target.matches('#rev-list li')) {
      const flow = e.target.getAttribute('data-flow');
      if (flow === '0') return;
      document.querySelector(`#flow-${flow}`)?.classList.remove('highlight');
      e.target.classList.remove('hover');
      for (const item of document.querySelectorAll(`#rev-list li[data-flow='${flow}']`)) {
        item.classList.remove('highlight');
      }
    } else if (e.target.matches('#rel-container .flow-group')) {
      e.target.classList.remove('highlight');
      const flow = e.target.getAttribute('data-flow');
      for (const item of document.querySelectorAll(`#rev-list li[data-flow='${flow}']`)) {
        item.classList.remove('highlight');
      }
    } else if (e.target.matches('#rel-container .flow-commit')) {
      const rev = e.target.getAttribute('data-rev');
      document.querySelector(`#rev-list li#commit-${rev}`)?.classList.remove('hover');
    }
  });
}
