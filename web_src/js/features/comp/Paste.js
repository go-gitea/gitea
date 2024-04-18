import {htmlEscape} from 'escape-goat';
import {POST} from '../../modules/fetch.js';
import {imageInfo} from '../../utils/image.js';
import {getPastedContent, replaceTextareaSelection} from '../../utils/dom.js';
import {isUrl} from '../../utils/url.js';

async function uploadFile(file, uploadUrl) {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const res = await POST(uploadUrl, {data: formData});
  return await res.json();
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

async function handleClipboardFiles(editor, dropzone, files, e) {
  const uploadUrl = dropzone.getAttribute('data-upload-url');
  const filesContainer = dropzone.querySelector('.files');

  if (!dropzone || !uploadUrl || !filesContainer || !files.length) return;

  e.preventDefault();
  e.stopPropagation();

  for (const file of files) {
    if (!file) continue;
    const name = file.name.slice(0, file.name.lastIndexOf('.'));

    const placeholder = `![${name}](uploading ...)`;
    editor.insertPlaceholder(placeholder);

    const {uuid} = await uploadFile(file, uploadUrl);
    const {width, dppx} = await imageInfo(file);

    const url = `/attachments/${uuid}`;
    let text;
    if (file.type?.startsWith('image/')) {
      if (width > 0 && dppx > 1) {
        // Scale down images from HiDPI monitors. This uses the <img> tag because it's the only
        // method to change image size in Markdown that is supported by all implementations.
        text = `<img width="${Math.round(width / dppx)}" alt="${htmlEscape(name)}" src="${htmlEscape(url)}">`;
      } else {
        text = `![${name}](${url})`;
      }
    } else {
      text = `[${name}](${url})`;
    }
    editor.replacePlaceholder(placeholder, text);

    file.uuid = uuid;
    dropzone.dropzone.emit('addedfile', file);
    if (file.type?.startsWith('image/')) {
      const imgSrc = `/attachments/${file.uuid}`;
      dropzone.dropzone.emit('thumbnail', file, imgSrc);
      dropzone.querySelector(`img[src='${CSS.escape(imgSrc)}']`).style.maxWidth = '100%';
    }
    dropzone.dropzone.emit('complete', file);
    const input = document.createElement('input');
    input.setAttribute('name', 'files');
    input.setAttribute('type', 'hidden');
    input.setAttribute('id', uuid);
    input.value = uuid;
    filesContainer.append(input);
  }
}

function handleClipboardText(textarea, text, e) {
  // when pasting links over selected text, turn it into [text](link), except when shift key is held
  const {value, selectionStart, selectionEnd, _shiftDown} = textarea;
  if (_shiftDown) return;
  const selectedText = value.substring(selectionStart, selectionEnd);
  const trimmedText = text.trim();
  if (selectedText && isUrl(trimmedText)) {
    e.stopPropagation();
    e.preventDefault();
    replaceTextareaSelection(textarea, `[${selectedText}](${trimmedText})`);
  }
}

export function initEasyMDEPaste(easyMDE, dropzone) {
  const pasteFunc = (e) => {
    const {files} = getPastedContent(e);
    if (files.length) {
      handleClipboardFiles(new CodeMirrorEditor(easyMDE.codemirror), dropzone, files, e);
    }
  };
  easyMDE.codemirror.on('paste', (_, e) => pasteFunc(e));
  easyMDE.codemirror.on('drop', (_, e) => pasteFunc(e));
}

export function initTextareaPaste(textarea, dropzone) {
  const pasteFunc = (e) => {
    const {files, text} = getPastedContent(e);
    if (files.length) {
      handleClipboardFiles(new TextareaEditor(textarea), dropzone, files, e);
    } else if (text) {
      handleClipboardText(textarea, text, e);
    }
  };
  textarea.addEventListener('paste', (e) => pasteFunc(e));
  textarea.addEventListener('drop', (e) => pasteFunc(e));
}
