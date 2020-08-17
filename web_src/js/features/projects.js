import {reloadIssuesActions} from './issuesutil.js';

const {csrf} = window.config;

export default async function initProject() {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const {Sortable} = await import(/* webpackChunkName: 'sortable' */ 'sortablejs');

  const colContainer = document.getElementById('board-container');
  let projectURL = '';
  let projectID = 0;
  if (colContainer && colContainer.dataset) {
    projectURL = colContainer.dataset.url;
    projectID = colContainer.dataset.projectid;
  }

  if (colContainer) {
    $('body').keyup((e) => {
      if (e.keyCode === 27) {
        $('#current-card-details').addClass('invisible');
        $('#current-card-details').html('');
        $('#current-card-details-input').html('');
      }
    });
  }

  $('#current-card-details').click((e) => {
    if (e.pageX >= 980 && e.pageY <= 320) {
      $('#current-card-details').addClass('invisible');
      $('#current-card-details').html('');
      $('#current-card-details-input').html('');
    }
  });

  // search as you type new issues
  $('#current-card-details-input').on('keyup', '#issue-search', async (e) => {
    const q = e.currentTarget.value;
    const repoURL = $('[data-repourl]').data('repourl');
    const response = await fetch(`/api/v1/repos${repoURL}/issues?state=open&q=${q}&exclude_project_id=${projectID}&render_emoji_title=true`, {
      method: 'GET',
      headers: {'X-Csrf-Token': csrf, 'Content-Type': 'application/json'}
    });
    const dataIssues = await response.json();

    let cards = '';
    if (Array.isArray(dataIssues)) {
      for (const issue of dataIssues) {
        const card = `
          <div class='card draggable-card board-card' data-priority='0' data-id='0' data-issueid='${issue.id}'>
            <div class='content'>
              <div class='header'>
                <a href='${repoURL}/${issue.pull_request ? 'pulls' : 'issues'}/${issue.number}' data-url='${repoURL}/${issue.pull_request ? 'pulls' : 'issues'}/${issue.number}?sidebar=true'>#${issue.number} ${issue.title}</a>
              </div>
              <div class='meta'></div>
              <div class='description'></div>
            </div>
          </div>
        `;
        cards += card;
      }
      $('#issue-results').html(
        `<div class='ui cards draggable-cards'>${cards}</div>`
      );
    }
    sortCards('.draggable-cards');
  });
  sortCards('.draggable-cards');

  if (colContainer) {
    new Sortable(colContainer, {
      group: 'cols',
      animation: 150,
      filter: '.ignore-elements',
      // Element dragging ended
      onEnd: async (e) => {
        const boards = [];
        for (const v of Object.values($(e.to))) {
          $(v).children().each((j, childColumn) => {
            const column = $(childColumn).data();
            if (column && column.columnId) {
              boards.push({
                id: column.columnId,
                priority: j
              });
            }
          });
        }
        await fetch(`${projectURL}/board/priority`, {
          method: 'PUT',
          headers: {'X-Csrf-Token': csrf, 'Content-Type': 'application/json'},
          body: JSON.stringify({boards})
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
      .on('click', function (e) {
        e.preventDefault();

        $.ajax({
          url: $(this).data('url'),
          data: JSON.stringify({title: projectTitleInput.val()}),
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
      data: JSON.stringify({title: boardTitle.val()}),
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

  $('#new-project-issue-item').on('click', (_e) => {
    $('#current-card-details').removeClass('invisible');

    $('#current-card-details').show();
    $('#current-card-details').html(`
      <div id='issue-results' class='ui cards draggable-cards'></div>
    `);
    $('#current-card-details-input').html(`
      <div class='ui search'>
        <input class='prompt' type='text' id='issue-search' placeholder='Filter issues'/>
      </div>
    `);
    sortCards('#issue-results');
  });

  function sortCards(selector) {
    $(selector).each((_i, eli) => {
      new Sortable(eli, {
        group: 'shared',
        filter: '.ignore-elements',
        animation: 150,
        onEnd: async (e) => {
          const cardsFrom = [];
          const cardsTo = [];
          $(e.from).each((_i, v) => {
            const column = $($(v)[0]).data();
            $(v)
              .children()
              .each((j, y) => {
                const card = $(y).data();
                if (card && card.id && e.oldDraggableIndex !== e.newDraggableIndex) {
                  cardsFrom.push({id: card.id, priority: j, ProjectBoardID: column.columnId
                  });
                }
              });
          });

          $(e.to).each((_i, v) => {
            const column = $($(v)[0]).data();
            $(v)
              .children()
              .each((j, y) => {
                const card = y.dataset;
                if (card && card.id) {
                  cardsTo.push({
                    id: parseInt(card.id),
                    priority: parseInt(j),
                    ProjectBoardID: parseInt(column.columnId),
                    issueid: parseInt(card.issueid),
                    projectID: parseInt(projectID)
                  });
                } else if (card && card.issueid && card.id === 0) {
                  cardsTo.push({
                    issueid: parseInt(card.issueid),
                    priority: parseInt(j),
                    ProjectBoardID: parseInt(column.columnId),
                    projectID: parseInt(projectID)
                  });
                }
              });
          });

          // Can't use await because onEnd is not async
          const response = await fetch(`${projectURL}/issue/priority`, {
            method: 'PUT',
            headers: {
              'X-Csrf-Token': csrf,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({
              issues: cardsTo.concat(cardsFrom)
            })
          });
          const data = await response.json();
          if (data && Array.isArray(data) && data.length > 0) {
            $(data).each((_i, x) => {$(`.board-column .cards [data-issueid=${x.IssueID}]`).attr('data-id', x.ID)});
          }
        }
      });
    });
  }

  // Show issue or pr in board sidebar
  $('.board-column').on('click', '.draggable-cards [data-url]', (e) => {
    e.preventDefault();
    $('#current-card-details').empty();
    $('#current-card-details-input').empty();
    const data = $(e.currentTarget).data();
    $('#current-card-details').removeClass('invisible');
    $('#current-card-details').show();
    reloadIssuesActions(data);
  });
}
