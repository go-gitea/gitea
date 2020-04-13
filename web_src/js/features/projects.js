export default async function initProject(csrf) {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const { Sortable } = await import(
    /* webpackChunkName: "sortable" */ 'sortablejs'
  );

  const boardColumns = document.getElementsByClassName('board-column');

  for (let i = 0; i < boardColumns.length; i++) {
    new Sortable(
      document.getElementById(
        boardColumns[i].getElementsByClassName('board')[0].id
      ),
      {
        group: 'shared',
        animation: 150,
        onAdd: e => {
          $.ajax(`${e.to.dataset.url}/${e.item.dataset.issue}`, {
            headers: {
              'X-Csrf-Token': csrf,
              'X-Remote': true,
            },
            contentType: 'application/json',
            type: 'POST',
            success: () => {
              // setTimeout(reload(),3000)
            },
          });
        },
      }
    );
  }

  $('.edit-project-board').each(function() {
    const modal = $(this);

    const projectTitle = $(this).find(
      '.content > .form > .field > .project-board-title'
    );

    $(this)
      .find('.content > .form > .actions > .red')
      .click(function(e) {
        e.preventDefault();
        $.ajax({
          url: $(this).data('url'),
          data: JSON.stringify({ title: projectTitle.val() }),
          headers: {
            'X-Csrf-Token': csrf,
            'X-Remote': true,
          },
          contentType: 'application/json',
          method: 'PUT',
        }).done(res => {
          modal.modal('hide');
          // setTimeout(window.location.reload(), 2000);
        });
      });
  });
}
