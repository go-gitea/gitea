import $ from 'jquery';
import {GET} from '../modules/fetch.js';

export function initRepoGraphGit() {
  const graphContainer = document.getElementById('git-graph-container');
  if (!graphContainer) return;

  $('#flow-color-monochrome').on('click', () => {
    $('#flow-color-monochrome').addClass('active');
    $('#flow-color-colored').removeClass('active');
    $('#git-graph-container').removeClass('colored').addClass('monochrome');
    const params = new URLSearchParams(window.location.search);
    params.set('mode', 'monochrome');
    const queryString = params.toString();
    if (queryString) {
      window.history.replaceState({}, '', `?${queryString}`);
    } else {
      window.history.replaceState({}, '', window.location.pathname);
    }
    $('.pagination a').each((_, that) => {
      const href = $(that).attr('href');
      if (!href) return;
      const url = new URL(href, window.location);
      const params = url.searchParams;
      params.set('mode', 'monochrome');
      url.search = `?${params.toString()}`;
      $(that).attr('href', url.href);
    });
  });
  $('#flow-color-colored').on('click', () => {
    $('#flow-color-colored').addClass('active');
    $('#flow-color-monochrome').removeClass('active');
    $('#git-graph-container').addClass('colored').removeClass('monochrome');
    $('.pagination a').each((_, that) => {
      const href = $(that).attr('href');
      if (!href) return;
      const url = new URL(href, window.location);
      const params = url.searchParams;
      params.delete('mode');
      url.search = `?${params.toString()}`;
      $(that).attr('href', url.href);
    });
    const params = new URLSearchParams(window.location.search);
    params.delete('mode');
    const queryString = params.toString();
    if (queryString) {
      window.history.replaceState({}, '', `?${queryString}`);
    } else {
      window.history.replaceState({}, '', window.location.pathname);
    }
  });
  const url = new URL(window.location);
  const params = url.searchParams;
  const updateGraph = () => {
    const queryString = params.toString();
    const ajaxUrl = new URL(url);
    ajaxUrl.searchParams.set('div-only', 'true');
    window.history.replaceState({}, '', queryString ? `?${queryString}` : window.location.pathname);
    $('#pagination').empty();
    $('#rel-container').addClass('gt-hidden');
    $('#rev-container').addClass('gt-hidden');
    $('#loading-indicator').removeClass('gt-hidden');
    (async () => {
      const response = await GET(String(ajaxUrl));
      const html = await response.text();
      const $div = $(html);
      $('#pagination').html($div.find('#pagination').html());
      $('#rel-container').html($div.find('#rel-container').html());
      $('#rev-container').html($div.find('#rev-container').html());
      $('#loading-indicator').addClass('gt-hidden');
      $('#rel-container').removeClass('gt-hidden');
      $('#rev-container').removeClass('gt-hidden');
    })();
  };
  const dropdownSelected = params.getAll('branch');
  if (params.has('hide-pr-refs') && params.get('hide-pr-refs') === 'true') {
    dropdownSelected.splice(0, 0, '...flow-hide-pr-refs');
  }

  $('#flow-select-refs-dropdown').dropdown('set selected', dropdownSelected);
  $('#flow-select-refs-dropdown').dropdown({
    clearable: true,
    fullTextSeach: 'exact',
    onRemove(toRemove) {
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
    onAdd(toAdd) {
      if (toAdd === '...flow-hide-pr-refs') {
        params.set('hide-pr-refs', true);
      } else {
        params.append('branch', toAdd);
      }
      updateGraph();
    },
  });
  $('#git-graph-container').on('mouseenter', '#rev-list li', (e) => {
    const flow = $(e.currentTarget).data('flow');
    if (flow === 0) return;
    $(`#flow-${flow}`).addClass('highlight');
    $(e.currentTarget).addClass('hover');
    $(`#rev-list li[data-flow='${flow}']`).addClass('highlight');
  });
  $('#git-graph-container').on('mouseleave', '#rev-list li', (e) => {
    const flow = $(e.currentTarget).data('flow');
    if (flow === 0) return;
    $(`#flow-${flow}`).removeClass('highlight');
    $(e.currentTarget).removeClass('hover');
    $(`#rev-list li[data-flow='${flow}']`).removeClass('highlight');
  });
  $('#git-graph-container').on('mouseenter', '#rel-container .flow-group', (e) => {
    $(e.currentTarget).addClass('highlight');
    const flow = $(e.currentTarget).data('flow');
    $(`#rev-list li[data-flow='${flow}']`).addClass('highlight');
  });
  $('#git-graph-container').on('mouseleave', '#rel-container .flow-group', (e) => {
    $(e.currentTarget).removeClass('highlight');
    const flow = $(e.currentTarget).data('flow');
    $(`#rev-list li[data-flow='${flow}']`).removeClass('highlight');
  });
  $('#git-graph-container').on('mouseenter', '#rel-container .flow-commit', (e) => {
    const rev = $(e.currentTarget).data('rev');
    $(`#rev-list li#commit-${rev}`).addClass('hover');
  });
  $('#git-graph-container').on('mouseleave', '#rel-container .flow-commit', (e) => {
    const rev = $(e.currentTarget).data('rev');
    $(`#rev-list li#commit-${rev}`).removeClass('hover');
  });
}
