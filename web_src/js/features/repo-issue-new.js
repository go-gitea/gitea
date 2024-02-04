import $ from 'jquery';
import {getComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

export function initRepoIssueNew() {
  const newIssuePage = $('.repository.issue.new');
  if (!newIssuePage.length) return;

  // this is set from board-notes in repo-projects.js
  const boardNoteTitle = sessionStorage.getItem('board-note-title');
  const boardNoteContent = sessionStorage.getItem('board-note-content');

  if (boardNoteTitle) {
    const issueTitle = newIssuePage.find('#issue_title');
    issueTitle.val(boardNoteTitle);
  }
  if (boardNoteContent) {
    // @TODO: find a better way to get the combobox
    const waitForComboMarkdownEditorInterval = setInterval(() => {
      const comboMarkdownEditorContainer = newIssuePage.find('.combo-markdown-editor');
      const comboMarkdownEditor = getComboMarkdownEditor(comboMarkdownEditorContainer);
      if (!comboMarkdownEditor) return;

      clearInterval(waitForComboMarkdownEditorInterval);

      comboMarkdownEditor.value(boardNoteContent);
    }, 100);
  }
}
