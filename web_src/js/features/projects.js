const {csrf} = window.config;

export default async function initProject() {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const {Sortable} = await import(/* webpackChunkName: "sortable" */'sortablejs');
  const boardColumns = document.getElementsByClassName('board-column');

  for (const column of boardColumns) {
    new Sortable(
      column.getElementsByClassName('board')[0],
      {
        group: 'shared',
        animation: 150,
        onAdd: (e) => {
          $.ajax(`${e.to.dataset.url}/${e.item.dataset.issue}`, {
            headers: {
              'X-Csrf-Token': csrf,
              'X-Remote': true,
            },
            contentType: 'application/json',
            type: 'POST',
            error: () => {
              // ToDo: move board back
            },
          });
        },
      }
    );
  }

  $('.edit-project-board').each(function () {
    const projectTitle = $(this).find(
      '.content > .form > .field > .project-board-title'
    );

    $(this)
      .find('.content > .form > .actions > .red')
      .on('click', function (e) {
        e.preventDefault();

        $.ajax({
          url: $(this).data('url'),
          data: JSON.stringify({title: projectTitle.val()}),
          headers: {
            'X-Csrf-Token': csrf,
            'X-Remote': true,
          },
          contentType: 'application/json',
          method: 'PUT',
        }).done(() => {
          projectTitle.closest('form').removeClass('dirty');
          setTimeout(window.location.reload(true), 2000);
        });
      });
  });

  $('.delete-project-board').each(function () {
    $(this).click(function (e) {
      e.preventDefault();

      $.ajax({
        url: $(this).data('url'),
        headers: {
          'X-Csrf-Token': csrf,
          'X-Remote': true,
        },
        contentType: 'application/json',
        method: 'DELETE',
      }).done(() => {
        setTimeout(window.location.reload(true), 2000);
      });
    });
  });

  $('#new_board_submit').click(function (e) {
    e.preventDefault();

    const boardTitle = $('#new_board');

    $.ajax({
      url: $(this).data('url'),
      data: JSON.stringify({title: boardTitle.val()}),
      headers: {
        'X-Csrf-Token': csrf,
        'X-Remote': true,
      },
      contentType: 'application/json',
      method: 'POST',
    }).done(() => {
      boardTitle.closest('form').removeClass('dirty');
      setTimeout(window.location.reload(true), 2000);
    });
  });
}
