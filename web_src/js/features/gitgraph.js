export default async function initGitGraph() {
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
