import '@github/markdown-toolbar-element';
import {random} from '../utils.js';
import attachTribute from './tribute.js';

const {AppSubUrl, csrf} = window.config;

async function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const res = await fetch($('#dropzone').data('upload-url'), {
    method: 'POST',
    headers: {'X-Csrf-Token': csrf},
    body: formData,
  });
  return await res.json();
}

function insertAtCursor(el, value) {
  if (el.selectionStart || el.selectionStart === 0) {
    const startPos = el.selectionStart;
    const endPos = el.selectionEnd;
    el.value = el.value.substring(0, startPos) + value + el.value.substring(endPos, el.value.length);
    el.selectionStart = startPos + value.length;
    el.selectionEnd = startPos + value.length;
  } else {
    el.value += value;
  }
}

function replaceAndKeepCursor(el, oldval, newval) {
  if (el.selectionStart || el.selectionStart === 0) {
    const startPos = el.selectionStart;
    const endPos = el.selectionEnd;
    el.value = el.value.replace(oldval, newval);
    el.selectionStart = startPos + newval.length - oldval.length;
    el.selectionEnd = endPos + newval.length - oldval.length;
  } else {
    el.value = el.value.replace(oldval, newval);
  }
}

function getPastedImages(e) {
  if (!e.clipboardData) return [];

  const files = [];
  for (const item of e.clipboardData.items || []) {
    if (!item.type || !item.type.startsWith('image/')) continue;
    files.push(item.getAsFile());
  }

  if (files.length) {
    e.preventDefault();
    e.stopPropagation();
  }
  return files;
}

function initImagePaste(el) {
  const files = el.closest('form')?.querySelector('.files');
  if (!files) return;

  el.addEventListener('paste', async (e) => {
    for (const img of getPastedImages(e)) {
      const name = img.name.substr(0, img.name.lastIndexOf('.'));
      insertAtCursor(this, `![${name}]()`);
      const data = await uploadFile(img);
      replaceAndKeepCursor(this, `![${name}]()`, `![${name}](${AppSubUrl}/attachments/${data.uuid})`);
      const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
      $('.files').append(input);
    }
  });
}

function initCtrlEnterSubmit(el) {
  el.addEventListener('keydown', (e) => {
    if (((e.ctrlKey && !e.altKey) || e.metaKey) && (e.keyCode === 13 || e.keyCode === 10)) {
      $(el).closest('form').trigger('submit');
    }
  });
}

export async function createMarkdownEditor(textarea) {
  if (!textarea) return;

  const id = `markdown-editor-${random()}`;
  textarea.id = id;

  initImagePaste(textarea);
  initCtrlEnterSubmit(textarea);
  attachTribute(textarea, {mentions: true, emoji: true});

  const toolbar = textarea.closest('form')?.querySelector('markdown-toolbar');
  if (toolbar) {
    toolbar.setAttribute('for', id);
  }
}
