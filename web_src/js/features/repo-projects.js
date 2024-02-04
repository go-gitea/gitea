import $ from 'jquery';
import {useLightTextOnBackground} from '../utils/color.js';
import tinycolor from 'tinycolor2';
import {createSortable} from '../modules/sortable.js';
import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

const {csrfToken} = window.config;

function updateIssueAndNoteCount(cards) {
  const column = $(cards).closest('.project-column');
  const cnt = column.find('.issue-card, .note-card').length;
  column.find('.project-column-issue-note-count').text(cnt);
}

function createNewColumn(url, form, data) {
  $.ajax({
    url,
    data: JSON.stringify(data),
    headers: {
      'X-Csrf-Token': csrfToken,
    },
    contentType: 'application/json',
    method: 'POST',
  }).done(() => {
    form.removeClass('dirty');
    window.location.reload();
  });
}

function moveIssue({item, from, to, oldIndex}) {
  const columnCards = to.getElementsByClassName('issue-card');
  updateIssueAndNoteCount(from);
  updateIssueAndNoteCount(to);

  const columnSorting = {
    issues: Array.from(columnCards, (card, i) => ({
      issueID: parseInt($(card).attr('data-issue')),
      sorting: i,
    })),
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
    },
  });
}

function moveNote({item, from, to, oldIndex}) {
  const columnCards = to.getElementsByClassName('note-card');
  updateIssueAndNoteCount(from);
  updateIssueAndNoteCount(to);

  const columnSorting = {
    boardNotes: Array.from(columnCards, (card, i) => ({
      boardNoteID: parseInt($(card).data('note')),
      sorting: i,
    })),
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
    },
  });
}

async function initRepoProjectSortable() {
  const els = document.querySelectorAll('#project-board > .board.sortable');
  if (!els.length) return;

  // the HTML layout is: #project-board > .board > .project-column .cards > .issue-card
  const mainBoard = els[0];
  let boardColumns = mainBoard.getElementsByClassName('project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    filter: '[data-id="0"]',
    animation: 150,
    ghostClass: 'card-ghost',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: () => {
      boardColumns = mainBoard.getElementsByClassName('project-column');
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
    const boardIssueCardList = boardColumn.getElementsByClassName('issue-cards')[0];
    const boardNoteCardList = boardColumn.getElementsByClassName('note-cards')[0];
    createSortable(boardIssueCardList, {
      group: 'issue-shared',
      animation: 150,
      ghostClass: 'card-ghost',
      onAdd: moveIssue,
      onUpdate: moveIssue,
      delayOnTouchOnly: true,
      delay: 500
    });
    createSortable(boardNoteCardList, {
      group: 'note-shared',
      animation: 150,
      ghostClass: 'card-ghost',
      onAdd: moveNote,
      onUpdate: moveNote,
      delayOnTouchOnly: true,
      delay: 500
    });
  }
}

export function initRepoProject() {
  const mainContent = $('.repository.projects');
  if (!mainContent.length) {
    return;
  }

  const modalButtons = mainContent.find('.project-column button[data-modal]');
  modalButtons.each(function() {
    const modalButton = $(this);
    const modalId = modalButton.data('modal');

    const markdownEditor = $(`${modalId} .combo-markdown-editor`);
    if (!markdownEditor.length) return;

    modalButton.one('click', () => {
      initComboMarkdownEditor(markdownEditor, {easyMDEOptions: {maxHeight: '50vh', minHeight: '50vh'}});
    });
  });

  const _promise = initRepoProjectSortable();

  $('.create-project-issue-from-note').each(function () {
    const anchorLink = $(this);
    const wrapperCard = anchorLink.closest('.note-card');
    const titleInput = wrapperCard.find('[name="title"]');
    const contentTextarea = wrapperCard.find('.markdown-text-editor');

    anchorLink.on('click', () => {
      sessionStorage.setItem('board-note-title', titleInput.val());
      sessionStorage.setItem('board-note-content', contentTextarea.val());
    });
  });

  $('.edit-project-column-modal').each(function () {
    const projectHeader = $(this).closest('.project-column-header');
    const projectTitleLabel = projectHeader.find('.project-column-title');
    const projectTitleInput = $(this).find('.project-column-title-input');
    const projectColorInput = $(this).find('#new_project_column_color');
    const boardColumn = $(this).closest('.project-column');

    if (boardColumn.css('backgroundColor')) {
      setLabelColor(projectHeader, rgbToHex(boardColumn.css('backgroundColor')));
    }

    $(this).find('.edit-project-column-button').on('click', function (e) {
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
        projectTitleLabel.contents().last().replaceWith(projectTitleInput.val());
        projectTitleInput.closest('form').removeClass('dirty');
        if (projectColorInput.val()) {
          setLabelColor(projectHeader, projectColorInput.val());
        }
        boardColumn.attr('style', `background: ${projectColorInput.val()}!important`);
        $('.ui.modal').modal('hide');
      });
    });
  });

  $('.default-project-column-modal').each(function () {
    const boardColumn = $(this).closest('.project-column');
    const showButton = $(boardColumn).find('.default-project-column-show');
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

  $('.show-delete-project-column-modal').each(function () {
    const deleteColumnModalId = $(this).data('modal');
    const deleteColumnUrl = $(this).data('url');

    const columnToDelete = $(this).closest('.project-column');
    const deleteColumnModal = $(deleteColumnModalId);

    const deleteColumnButton = deleteColumnModal.find('.actions > .ok.button');
    deleteColumnButton.on('click', (e) => {
      e.preventDefault();

      $.ajax({
        url: deleteColumnUrl,
        headers: {
          'X-Csrf-Token': csrfToken,
        },
        contentType: 'application/json',
        method: 'DELETE',
      }).done(() => {
        columnToDelete.remove();
      });
    });
  });

  $('#new_project_column_submit').on('click', function (e) {
    e.preventDefault();
    const columnTitle = $('#new_project_column');
    const projectColorInput = $('#new_project_column_color_picker');
    if (!columnTitle.val()) {
      return;
    }
    const url = $(this).data('url');
    const form = columnTitle.closest('form');
    createNewColumn(url, form, {title: columnTitle.val(), color: projectColorInput.val()});
  });
}

function setLabelColor(label, color) {
  const {r, g, b} = tinycolor(color).toRgb();
  if (useLightTextOnBackground(r, g, b)) {
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
