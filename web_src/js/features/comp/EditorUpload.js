import {htmlEscape} from 'escape-goat';
import {imageInfo} from '../../utils/image.js';
import {replaceTextareaSelection} from '../../utils/dom.js';
import {isUrl} from '../../utils/url.js';
import {isWellKnownImageFilename} from '../../utils.js';
import {triggerEditorContentChanged} from './EditorMarkdown.js';
import {DropzoneCustomEventRemovedFile} from '../dropzone.js';

let uploadIdCounter = 0;

function uploadFile(dropzoneEl, file) {
  return new Promise((resolve, _) => {
    file._giteaUploadId = uploadIdCounter++;
    const dropzoneInst = dropzoneEl.dropzone;
    const onSuccess = (successFile, successResp) => {
      if (successFile._giteaUploadId === file._giteaUploadId) {
        resolve({uuid: successResp.uuid});
      }
      dropzoneInst.off('success', onSuccess);
    };
    // TODO: handle errors (or maybe not needed at the moment)
    dropzoneInst.on('success', onSuccess);
    dropzoneInst.handleFiles([file]);
  });
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

function isImageFile(file) {
  return file.type?.startsWith('image/') || isWellKnownImageFilename(file.name);
}

async function handleUploadFiles(editor, dropzoneEl, files, e) {
  e.preventDefault();
  for (const file of files) {
    const name = file.name.slice(0, file.name.lastIndexOf('.'));
    const isImage = isImageFile(file);

    let placeholder = `[${name}](uploading ...)`;
    if (isImage) placeholder = `!${placeholder}`;

    editor.insertPlaceholder(placeholder);
    const {uuid} = await uploadFile(dropzoneEl, file);

    let fileMarkdownLink;
    if (isImage) {
      const {width, dppx} = await imageInfo(file);
      if (width > 0 && dppx > 1) {
        // Scale down images from HiDPI monitors. This uses the <img> tag because it's the only
        // method to change image size in Markdown that is supported by all implementations.
        // Make the image link relative to the repo path, then the final URL is "/sub-path/owner/repo/attachments/{uuid}"
        const url = `attachments/${uuid}`;
        fileMarkdownLink = `<img width="${Math.round(width / dppx)}" alt="${htmlEscape(name)}" src="${htmlEscape(url)}">`;
      } else {
        // Markdown always renders the image with a relative path, so the final URL is "/sub-path/owner/repo/attachments/{uuid}"
        // TODO: it should also use relative path for consistency, because absolute is ambiguous for "/sub-path/attachments" or "/attachments"
        const url = `/attachments/${uuid}`;
        fileMarkdownLink = `![${name}](${url})`;
      }
    } else {
      const url = `/attachments/${uuid}`;
      fileMarkdownLink = `[${name}](${url})`;
    }
    editor.replacePlaceholder(placeholder, fileMarkdownLink);
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
  if (selectedText && isUrl(trimmedText)) {
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

export function initTextareaUpload(textarea, dropzoneEl) {
  let isShiftDown = false;
  textarea.addEventListener('keydown', (e) => {
    if (e.shiftKey) isShiftDown = true;
  });
  textarea.addEventListener('keyup', (e) => {
    if (!e.shiftKey) isShiftDown = false;
  });
  textarea.addEventListener('paste', (e) => {
    const {images, text} = getPastedContent(e);
    if (images.length) {
      handleUploadFiles(new TextareaEditor(textarea), dropzoneEl, images, e);
    } else if (text) {
      handleClipboardText(textarea, e, {text, isShiftDown});
    }
  });
  textarea.addEventListener('drop', (e) => {
    if (!e.dataTransfer.files.length) return;
    handleUploadFiles(new TextareaEditor(textarea), dropzoneEl, e.dataTransfer.files, e);
  });
  dropzoneEl.dropzone.on(DropzoneCustomEventRemovedFile, ({fileUuid}) => {
    const newText = removeAttachmentLinksFromMarkdown(textarea.value, fileUuid);
    if (textarea.value !== newText) textarea.value = newText;
  });
}
