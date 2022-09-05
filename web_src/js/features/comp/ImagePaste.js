import $ from 'jquery';
import {getAttachedEasyMDE} from './EasyMDE.js';

/**
 * @param editor{EasyMDE}
 * @param fileUuid
 */
export function removeUploadedFileFromEditor(editor, fileUuid) {
  // the raw regexp is: /!\[[^\]]*]\(\/attachments\/{uuid}\)/ for remove file text in textarea
  if (editor && editor.editor) {
    const re = new RegExp(`(!|)\\[[^\\]]*]\\(/attachments/${fileUuid}\\)`);
    if (editor.editor.setValue) {
      editor.editor.setValue(editor.editor.getValue().replace(re, '')); // at the moment, we assume the editor is an EasyMDE
    } else {
      editor.editor.value = editor.editor.value.replace(re, '');
    }
  }
}

function clipboardPastedFiles(e) {
  const data = e.clipboardData || e.dataTransfer;
  if (!data) return [];

  const files = [];
  const datafiles = e.clipboardData?.items || e.dataTransfer?.files;
  for (const item of datafiles || []) {
    const file = e.clipboardData ? item.getAsFile() : item;
    if (file === null || !item.type) continue;
    files.push(file);
  }
  return files;
}


class TextareaEditor {
  constructor(editor) {
    this.editor = editor;
  }

  insertPlaceholder(value) {
    const editor = this.editor;
    const startPos = editor.selectionStart;
    const endPos = editor.selectionEnd;
    editor.value = editor.value.substring(0, startPos) + value + editor.value.substring(endPos);
    editor.selectionStart = startPos;
    editor.selectionEnd = startPos + value.length;
    editor.focus();
  }

  replacePlaceholder(oldVal, newVal) {
    const editor = this.editor;
    const startPos = editor.selectionStart;
    const endPos = editor.selectionEnd;
    if (editor.value.substring(startPos, endPos) === oldVal) {
      editor.value = editor.value.substring(0, startPos) + newVal + editor.value.substring(endPos);
      editor.selectionEnd = startPos + newVal.length;
    } else {
      editor.value = editor.value.replace(oldVal, newVal);
      editor.selectionEnd -= oldVal.length;
      editor.selectionEnd += newVal.length;
    }
    editor.selectionStart = editor.selectionEnd;
    editor.focus();
  }
}

class CodeMirrorEditor {
  constructor(editor) {
    this.editor = editor;
  }

  insertPlaceholder(value) {
    const editor = this.editor;
    const startPoint = editor.getCursor('start');
    const endPoint = editor.getCursor('end');
    editor.replaceSelection(value);
    endPoint.ch = startPoint.ch + value.length;
    editor.setSelection(startPoint, endPoint);
    editor.focus();
  }

  replacePlaceholder(oldVal, newVal) {
    const editor = this.editor;
    const endPoint = editor.getCursor('end');
    if (editor.getSelection() === oldVal) {
      editor.replaceSelection(newVal);
    } else {
      editor.setValue(editor.getValue().replace(oldVal, newVal));
    }
    endPoint.ch -= oldVal.length;
    endPoint.ch += newVal.length;
    editor.setSelection(endPoint, endPoint);
    editor.focus();
  }
}

export function initEasyMDEFilePaste(easyMDE, $dropzone) {
  if ($dropzone.length !== 1) throw new Error('invalid dropzone binding for editor');

  const uploadUrl = $dropzone.attr('data-upload-url');
  const $files = $dropzone.find('.files');

  if (!uploadUrl || !$files.length) return;

  const uploadClipboardImage = async (editor, e) => {
    const pastedImages = clipboardPastedFiles(e);
    if (!pastedImages || pastedImages.length === 0) {
      return;
    }
    e.preventDefault();
    e.stopPropagation();

    for (const img of pastedImages) {
      img.editor = editor;
      $dropzone[0].dropzone.addFile(img);
    }
  };

  easyMDE.codemirror.on('paste', async (_, e) => {
    return uploadClipboardImage(new CodeMirrorEditor(easyMDE.codemirror), e);
  });

  easyMDE.codemirror.on('drop', async (_, e) => {
    return uploadClipboardImage(new CodeMirrorEditor(easyMDE.codemirror), e);
  });

  $(easyMDE.element).on('paste drop', async (e) => {
    return uploadClipboardImage(new TextareaEditor(easyMDE.element), e.originalEvent);
  });
}

export async function addUploadedFileToEditor(file) {
  if (!file.editor) {
    const form = file.previewElement.closest('div.comment');
    if (form) {
      const editor = getAttachedEasyMDE(form.querySelector('textarea'));
      if (editor) {
        if (editor.codemirror) {
          file.editor = new CodeMirrorEditor(editor.codemirror);
        } else {
          file.editor = new TextareaEditor(editor);
        }
      }
      if (file.editor) {
        const name = file.name.slice(0, file.name.lastIndexOf('.'));
        const placeholder = `![${name}](uploading ...)`;
        file.editor.insertPlaceholder(placeholder);
      }
    }
  }
}
