import $ from 'jquery';
import {POST} from '../../modules/fetch.js';

async function uploadFile(file, uploadUrl) {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const res = await POST(uploadUrl, {data: formData});
  return await res.json();
}

function clipboardPastedImages(e) {
  if (!e.clipboardData) return [];

  const files = [];
  for (const item of e.clipboardData.items || []) {
    if (!item.type || !item.type.startsWith('image/')) continue;
    files.push(item.getAsFile());
  }
  return files;
}

function triggerEditorContentChanged(target) {
  target.dispatchEvent(new CustomEvent('ce-editor-content-changed', {bubbles: true}));
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
    triggerEditorContentChanged(editor);
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
    triggerEditorContentChanged(editor);
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
    triggerEditorContentChanged(editor.getTextArea());
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
    triggerEditorContentChanged(editor.getTextArea());
  }
}

const uploadClipboardImage = async (editor, dropzone, e) => {
  const $dropzone = $(dropzone);
  const uploadUrl = $dropzone.attr('data-upload-url');
  const $files = $dropzone.find('.files');

  if (!uploadUrl || !$files.length) return;

  const pastedImages = clipboardPastedImages(e);
  if (!pastedImages || pastedImages.length === 0) {
    return;
  }
  e.preventDefault();
  e.stopPropagation();

  for (const img of pastedImages) {
    const name = img.name.slice(0, img.name.lastIndexOf('.'));

    const placeholder = `![${name}](uploading ...)`;
    editor.insertPlaceholder(placeholder);
    const data = await uploadFile(img, uploadUrl);
    editor.replacePlaceholder(placeholder, `![${name}](/attachments/${data.uuid})`);

    const $input = $(`<input name="files" type="hidden">`).attr('id', data.uuid).val(data.uuid);
    $files.append($input);
  }
};

export function initEasyMDEImagePaste(easyMDE, dropzone) {
  if (!dropzone) return;
  easyMDE.codemirror.on('paste', async (_, e) => {
    return uploadClipboardImage(new CodeMirrorEditor(easyMDE.codemirror), dropzone, e);
  });
}

export function initTextareaImagePaste(textarea, dropzone) {
  if (!dropzone) return;
  $(textarea).on('paste', async (e) => {
    return uploadClipboardImage(new TextareaEditor(textarea), dropzone, e.originalEvent);
  });
}
