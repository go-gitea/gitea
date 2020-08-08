const { csrf } = window.config;

export default async function initProject() {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const { Sortable } = await import(
    /* webpackChunkName: 'sortable' */ 'sortablejs'
  );
  const boardColumns = document.getElementsByClassName('board-column');
  const colContainer = document.getElementById('board-container');
  let projectURL = '';
  let projectID = 0;
  if (colContainer && colContainer.dataset) {
    projectURL = colContainer.dataset.url;
    projectID = colContainer.dataset.projectid;
  }

  //search as you type new issues
  $('#current-card-details-input').on('keyup', '#issue-search', e => {
  let q = e.currentTarget.value;
  let repoURL = $('[data-repourl]').data('repourl');
  fetch(`/api/v1/repos${repoURL}/issues?state=open&q=${q}&not_in_project_id=${projectID}`, {
    method: 'GET',
    headers: { 'X-Csrf-Token': csrf, 'Content-Type': 'application/json' }
  })
    .then(function(res) {
      return res.json();
    })
    .then(function(data) {
      let cards = '';
      data.map(issue => {
        let card = `<div class='card draggable-card board-card' data-priority='0' data-id='0' data-issueid='${issue.id}'>
      <div class='content'>
        <div class='header'>
                <a href='${repoURL}/${issue.IsPull? 'pulls' : 'issues'}/${issue.number}/sidebar/true'
                  data-url='${repoURL}/${issue.IsPull? 'pulls' : 'issues'}/${issue.number}/sidebar/true'>
        #${issue.number} ${issue.title}
        </a>
        </div>
        <div class='meta'>
        </div>
        <div class='description'>

        </div>
      </div>
    </div>`;
        cards += card;
      });
      $('#issue-results').html(
        `<div class='ui cards draggable-cards'>${cards}</div>`
      );
      sortCards('.draggable-cards');
    });
  });
  sortCards('.draggable-cards')

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
                  priority: j
                });
              }
            });
        });
        fetch(`${projectURL}/updatePriorities`, {
          method: 'PUT',
          headers: { 'X-Csrf-Token': csrf, 'Content-Type': 'application/json' },
          body: JSON.stringify({
            boards: columns
          })
        })
          .then(function(res) {
            return res.json();
          })
          .then(function(data) {
            console.log(JSON.stringify(data));
          });
      }
    });
  }

  $('.edit-project-board').each(function() {
    const projectTitleLabel = $(this)
      .closest('.board-column-header')
      .find('.board-label');
    const projectTitleInput = $(this).find(
      '.content > .form > .field > .project-board-title'
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
            'X-Remote': true
          },
          contentType: 'application/json',
          method: 'PUT'
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
          'X-Remote': true
        },
        contentType: 'application/json',
        method: 'DELETE'
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
        'X-Remote': true
      },
      contentType: 'application/json',
      method: 'POST'
    }).done(() => {
      boardTitle.closest('form').removeClass('dirty');
      setTimeout(window.location.reload(true), 2000);
    });
  });

  $('#new-project-issue-item').on('click', e => {
    $('#current-card-details').removeClass('hide');
    $('#current-card-details').css('visibility', 'visible');

    $('#current-card-details').show();
    let searchHtml= `<div class='ui search'>
      <input class='prompt' type='text' id='issue-search' placeholder='Filter issues' /></div>`
    let html = `
      <div id='issue-results' class='ui cards draggable-cards'></div>
      `;
    $('#current-card-details').html(html);
    $('#current-card-details-input').html(searchHtml);
    sortCards('#issue-results')
  });

  function sortCards(selector){
    $(selector).each(function(i, eli) {
      new Sortable(eli, {
        group: 'shared',
        filter: '.ignore-elements',
        animation: 150,
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
                    ProjectBoardID: column.columnId
                  });
              });
          });

          $(evt.to).each((i, v) => {
            let column = $($(v)[0]).data();
            $(v)
              .children()
              .each((j, y) => {
                let card = y.dataset;
                if (card && card.id) {
                  cardsTo.push({
                    id: parseInt(card.id),
                    priority: parseInt(j),
                    ProjectBoardID: parseInt(column.columnId),
                    issueid: parseInt(card.issueid),
                    projectID: parseInt(projectID)
                  });
                } else if (card && card.issueid && card.id === 0){
                  cardsTo.push({
                    issueid: parseInt(card.issueid),
                    priority: parseInt(j),
                    ProjectBoardID: parseInt(column.columnId),
                    projectID: parseInt(projectID)
                  })
                }
              });
          });
          fetch(`${projectURL}/updateIssuesPriorities`, {
            method: 'PUT',
            headers: {
              'X-Csrf-Token': csrf,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({
              issues: cardsTo.concat(cardsFrom)
            })
          })
            .then(function(res) {
              return res.json();
            })
            .then(function(data) {
              if(data && data.length >0){
                $(data).each((i,x)=>{$(`.board-column .cards [data-issueid=${x.IssueID}]`).attr('data-id', x.ID)})
              }
            });
        }
      });
    });
  }
}

