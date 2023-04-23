import $ from 'jquery';
import {useLightTextOnBackground} from '../utils.js';

const {csrfToken} = window.config;

function updateIssueCount(cards) {
  const parent = cards.parentElement;
  const cnt = parent.getElementsByClassName('board-card').length;
  parent.getElementsByClassName('board-card-cnt')[0].innerText = cnt;
}

function moveIssue({item, from, to, oldIndex}) {
  const columnCards = to.getElementsByClassName('board-card');
  updateIssueCount(from);
  updateIssueCount(to);

  const columnSorting = {
    issues: [...columnCards].map((card, i) => ({
      issueID: parseInt($(card).attr('data-issue')),
      sorting: i
    }))
  };

  $.ajax({
    url: `${to.getAttribute('data-url')}/move`,
    data: JSON.stringify(columnSorting),
    headers: {
      'X-Csrf-Token': csrfToken,
    },
    contentType: 'application/json',
    type: 'POST',
    error: () => {
      from.insertBefore(item, from.children[oldIndex]);
    }
  });
}

async function initRepoProjectSortable() {
  const els = document.querySelectorAll('#project-board > .board');
  if (!els.length) return;

  const {Sortable} = await import(/* webpackChunkName: "sortable" */'sortablejs');

  // the HTML layout is: #project-board > .board > .board-column .board.cards > .board-card.card .content
  const mainBoard = els[0];
  let boardColumns = mainBoard.getElementsByClassName('board-column');
  new Sortable(mainBoard, {
    group: 'board-column',
    draggable: '.board-column',
    filter: '[data-id="0"]',
    animation: 150,
    ghostClass: 'card-ghost',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: () => {
      boardColumns = mainBoard.getElementsByClassName('board-column');
      for (let i = 0; i < boardColumns.length; i++) {
        const column = boardColumns[i];
        if (parseInt($(column).data('sorting')) !== i) {
          $.ajax({
            url: $(column).data('url'),
            data: JSON.stringify({sorting: i, color: rgbToHex($(column).css('backgroundColor'))}),
            headers: {
              'X-Csrf-Token': csrfToken,
            },
            contentType: 'application/json',
            method: 'PUT',
          });
        }
      }
    },
  });

  for (const boardColumn of boardColumns) {
    const boardCardList = boardColumn.getElementsByClassName('board')[0];
    new Sortable(boardCardList, {
      group: 'shared',
      animation: 150,
      ghostClass: 'card-ghost',
      onAdd: moveIssue,
      onUpdate: moveIssue,
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

export function initRepoProject() {
  if (!$('.repository.projects').length) {
    return;
  }

  const _promise = initRepoProjectSortable();

  $('.edit-project-board').each(function () {
    const projectHeader = $(this).closest('.board-column-header');
    const projectTitleLabel = projectHeader.find('.board-label');
    const projectTitleInput = $(this).find('.project-board-title');
    const projectColorInput = $(this).find('#new_board_color');
    const boardColumn = $(this).closest('.board-column');

    if (boardColumn.css('backgroundColor')) {
      setLabelColor(projectHeader, rgbToHex(boardColumn.css('backgroundColor')));
    }

    $(this).find('.edit-column-button').on('click', function (e) {
      e.preventDefault();

      $.ajax({
        url: $(this).data('url'),
        data: JSON.stringify({title: projectTitleInput.val(), color: projectColorInput.val()}),
        headers: {
          'X-Csrf-Token': csrfToken,
        },
        contentType: 'application/json',
        method: 'PUT',
      }).done(() => {
        projectTitleLabel.text(projectTitleInput.val());
        projectTitleInput.closest('form').removeClass('dirty');
        if (projectColorInput.val()) {
          setLabelColor(projectHeader, projectColorInput.val());
        }
        boardColumn.attr('style', `background: ${projectColorInput.val()}!important`);
        $('.ui.modal').modal('hide');
      });
    });
  });

  $('.default-project-board-modal').each(function () {
    const boardColumn = $(this).closest('.board-column');
    const showButton = $(boardColumn).find('.default-project-board-show');
    const commitButton = $(this).find('.actions > .ok.button');

    $(commitButton).on('click', (e) => {
      e.preventDefault();

      $.ajax({
        method: 'POST',
        url: $(showButton).data('url'),
        headers: {
          'X-Csrf-Token': csrfToken,
        },
        contentType: 'application/json',
      }).done(() => {
        window.location.reload();
      });
    });
  });

  $('.show-delete-column-modal').each(function () {
    const deleteColumnModal = $(`${$(this).attr('data-modal')}`);
    const deleteColumnButton = deleteColumnModal.find('.actions > .ok.button');
    const deleteUrl = $(this).attr('data-url');

    deleteColumnButton.on('click', (e) => {
      e.preventDefault();

      $.ajax({
        url: deleteUrl,
        headers: {
          'X-Csrf-Token': csrfToken,
        },
        contentType: 'application/json',
        method: 'DELETE',
      }).done(() => {
        window.location.reload();
      });
    });
  });

  $('#new_board_submit').on('click', function (e) {
    e.preventDefault();

    const boardTitle = $('#new_board');
    const projectColorInput = $('#new_board_color_picker');

    $.ajax({
      url: $(this).data('url'),
      data: JSON.stringify({title: boardTitle.val(), color: projectColorInput.val()}),
      headers: {
        'X-Csrf-Token': csrfToken,
      },
      contentType: 'application/json',
      method: 'POST',
    }).done(() => {
      boardTitle.closest('form').removeClass('dirty');
      window.location.reload();
    });
  });
}

function setLabelColor(label, color) {
  if (useLightTextOnBackground(color)) {
    label.removeClass('dark-label').addClass('light-label');
  } else {
    label.removeClass('light-label').addClass('dark-label');
  }
}

function rgbToHex(rgb) {
  rgb = rgb.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+).*\)$/);
  return `#${hex(rgb[1])}${hex(rgb[2])}${hex(rgb[3])}`;
}

function hex(x) {
  const hexDigits = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'];
  return Number.isNaN(x) ? '00' : hexDigits[(x - x % 16) / 16] + hexDigits[x % 16];
}
