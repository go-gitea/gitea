import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

export function initComboMarkdownEditorGlobal() {
  const markdownEditor = document.getElementsByClassName('combo-markdown-editor-init');

  if (markdownEditor.length > 0) {
    initComboMarkdownEditor(markdownEditor[0]);
  }
}
