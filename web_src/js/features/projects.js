const { csrf } = window.config;

export default async function initProject() {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const { Sortable } = await import(
    /* webpackChunkName: "sortable" */ 'sortablejs'
  );
  const boardColumns = document.getElementsByClassName('board-column');
  const colContainer = document.getElementById('board-container');
  let projectURL = '';
  if (colContainer && colContainer.dataset) {
    projectURL = colContainer.dataset.url;
  }
  $('.draggable-cards').each(function(i, eli) {
    new Sortable(eli, {
      group: 'shared',
      filter: '.ignore-elements',
      animation: 150,
      // Element dragging ended
      onEnd: function(/**Event*/ evt) {
        var itemEl = evt.item; // dragged HTMLElement
        let cardsFrom = [];
        let cardsTo = [];
        $(evt.from).each((i, v) => {
          let column = $($(v)[0]).data();
          $(v)
            .children()
            .each((j, y) => {
              let card = $(y).data();
              if (
                card &&
                card.id &&
                evt.oldDraggableIndex !== evt.newDraggableIndex
              )
                cardsFrom.push({
                  id: card.id,
                  priority: j,
                  ProjectBoardID: column.columnId,
                });
            });
        });

        $(evt.to).each((i, v) => {
          let column = $($(v)[0]).data();
          $(v)
            .children()
            .each((j, y) => {
              let card = $(y).data();
              if (card && card.id) {
                cardsTo.push({
                  id: card.id,
                  priority: j,
                  ProjectBoardID: column.columnId,
                });
              }
            });
        });
        fetch(`${projectURL}/updateIssuesPriorities`, {
          method: 'PUT',
          headers: {
            'X-Csrf-Token': csrf,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            issues: cardsTo.concat(cardsFrom),
          }),
        })
          .then(function(res) {
            return res.json();
          })
          .then(function(data) {
            console.log(JSON.stringify(data));
          });
      },
    });
  });

  if (colContainer) {
    new Sortable(colContainer, {
      group: 'cols',
      animation: 150,
      filter: '.ignore-elements',
      // Element dragging ended
      onEnd: function(/**Event*/ evt) {
        var itemEl = evt.item; // dragged HTMLElement
        let columns = [];
        $(evt.to).each((i, v) => {
          $(v)
            .children()
            .each((j, y) => {
              let column = $(y).data();
              if (column && column.columnId) {
                columns.push({
                  id: column.columnId,
                  priority: j,
                });
              }
            });
        });
        fetch(`${projectURL}/updatePriorities`, {
          method: 'PUT',
          headers: { 'X-Csrf-Token': csrf, 'Content-Type': 'application/json' },
          body: JSON.stringify({
            boards: columns,
          }),
        })
          .then(function(res) {
            return res.json();
          })
          .then(function(data) {
            console.log(JSON.stringify(data));
          });
      },
    });
  }
  $('.edit-project-board').each(function() {
    const projectTitleLabel = $(this)
      .closest('.board-column-header')
      .find('.board-label');
    const projectTitleInput = $(this).find(
      '.content > .form > .field > .project-board-title',
    );

    $(this)
      .find('.content > .form > .actions > .red')
      .on('click', function(e) {
        e.preventDefault();

        $.ajax({
          url: $(this).data('url'),
          data: JSON.stringify({ title: projectTitleInput.val() }),
          headers: {
            'X-Csrf-Token': csrf,
            'X-Remote': true,
          },
          contentType: 'application/json',
          method: 'PUT',
        }).done(() => {
          projectTitleLabel.text(projectTitleInput.val());
          projectTitleInput.closest('form').removeClass('dirty');
          $('.ui.modal').modal('hide');
        });
      });
  });

  $('.delete-project-board').each(function() {
    $(this).click(function(e) {
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

  $('#new_board_submit').click(function(e) {
    e.preventDefault();

    const boardTitle = $('#new_board');

    $.ajax({
      url: $(this).data('url'),
      data: JSON.stringify({ title: boardTitle.val() }),
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
