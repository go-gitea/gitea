import {imageInfo} from '../../utils/image.ts';
import {replaceTextareaSelection} from '../../utils/dom.ts';
import {isUrl} from '../../utils/url.ts';
import {textareaInsertText, triggerEditorContentChanged} from './EditorMarkdown.ts';
import {
  DropzoneCustomEventRemovedFile,
  DropzoneCustomEventUploadDone,
  generateMarkdownLinkForAttachment,
} from '../dropzone.ts';
import type CodeMirror from 'codemirror';

let uploadIdCounter = 0;

export const EventUploadStateChanged = 'ce-upload-state-changed';

export function triggerUploadStateChanged(target) {
  target.dispatchEvent(new CustomEvent(EventUploadStateChanged, {bubbles: true}));
}

function uploadFile(dropzoneEl, file) {
  return new Promise((resolve) => {
    const curUploadId = uploadIdCounter++;
    file._giteaUploadId = curUploadId;
    const dropzoneInst = dropzoneEl.dropzone;
    const onUploadDone = ({file}) => {
      if (file._giteaUploadId === curUploadId) {
        dropzoneInst.off(DropzoneCustomEventUploadDone, onUploadDone);
        resolve(file);
      }
    };
    dropzoneInst.on(DropzoneCustomEventUploadDone, onUploadDone);
    dropzoneInst.handleFiles([file]);
  });
}

class TextareaEditor {
  editor : HTMLTextAreaElement;

  constructor(editor) {
    this.editor = editor;
  }

  insertPlaceholder(value) {
    textareaInsertText(this.editor, value);
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
  editor: CodeMirror.EditorFromTextArea;

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

async function handleUploadFiles(editor, dropzoneEl, files, e) {
  e.preventDefault();
  for (const file of files) {
    const name = file.name.slice(0, file.name.lastIndexOf('.'));
    const {width, dppx} = await imageInfo(file);
    const placeholder = `[${name}](uploading ...)`;

    editor.insertPlaceholder(placeholder);
    await uploadFile(dropzoneEl, file); // the "file" will get its "uuid" during the upload
    editor.replacePlaceholder(placeholder, generateMarkdownLinkForAttachment(file, {width, dppx}));
  }
}

export function removeAttachmentLinksFromMarkdown(text, fileUuid) {
  text = text.replace(new RegExp(`!?\\[([^\\]]+)\\]\\(/?attachments/${fileUuid}\\)`, 'g'), '');
  text = text.replace(new RegExp(`<img[^>]+src="/?attachments/${fileUuid}"[^>]*>`, 'g'), '');
  return text;
}

function handleClipboardText(textarea, e, {text, isShiftDown}) {
  // pasting with "shift" means "paste as original content" in most applications
  if (isShiftDown) return; // let the browser handle it

  // when pasting links over selected text, turn it into [text](link)
  const {value, selectionStart, selectionEnd} = textarea;
  const selectedText = value.substring(selectionStart, selectionEnd);
  const trimmedText = text.trim();
  if (selectedText && isUrl(trimmedText) && !isUrl(selectedText)) {
    e.preventDefault();
    replaceTextareaSelection(textarea, `[${selectedText}](${trimmedText})`);
  }
  // else, let the browser handle it
}

// extract text and images from "paste" event
function getPastedContent(e) {
  const images = [];
  for (const item of e.clipboardData?.items ?? []) {
    if (item.type?.startsWith('image/')) {
      images.push(item.getAsFile());
    }
  }
  const text = e.clipboardData?.getData?.('text') ?? '';
  return {text, images};
}

export function initEasyMDEPaste(easyMDE, dropzoneEl) {
  const editor = new CodeMirrorEditor(easyMDE.codemirror);
  easyMDE.codemirror.on('paste', (_, e) => {
    const {images} = getPastedContent(e);
    if (!images.length) return;
    handleUploadFiles(editor, dropzoneEl, images, e);
  });
  easyMDE.codemirror.on('drop', (_, e) => {
    if (!e.dataTransfer.files.length) return;
    handleUploadFiles(editor, dropzoneEl, e.dataTransfer.files, e);
  });
  dropzoneEl.dropzone.on(DropzoneCustomEventRemovedFile, ({fileUuid}) => {
    const oldText = easyMDE.codemirror.getValue();
    const newText = removeAttachmentLinksFromMarkdown(oldText, fileUuid);
    if (oldText !== newText) easyMDE.codemirror.setValue(newText);
  });
}

export function initTextareaEvents(textarea, dropzoneEl) {
  let isShiftDown = false;
  textarea.addEventListener('keydown', (e) => {
    if (e.shiftKey) isShiftDown = true;
  });
  textarea.addEventListener('keyup', (e) => {
    if (!e.shiftKey) isShiftDown = false;
  });
  textarea.addEventListener('paste', (e) => {
    const {images, text} = getPastedContent(e);
    if (images.length && dropzoneEl) {
      handleUploadFiles(new TextareaEditor(textarea), dropzoneEl, images, e);
    } else if (text) {
      handleClipboardText(textarea, e, {text, isShiftDown});
    }
  });
  textarea.addEventListener('drop', (e) => {
    if (!e.dataTransfer.files.length) return;
    if (!dropzoneEl) return;
    handleUploadFiles(new TextareaEditor(textarea), dropzoneEl, e.dataTransfer.files, e);
  });
  dropzoneEl?.dropzone.on(DropzoneCustomEventRemovedFile, ({fileUuid}) => {
    const newText = removeAttachmentLinksFromMarkdown(textarea.value, fileUuid);
    if (textarea.value !== newText) textarea.value = newText;
  });
}
