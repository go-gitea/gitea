import $ from 'jquery';
const {csrfToken} = window.config;

async function uploadFile(file, uploadUrl) {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const res = await fetch(uploadUrl, {
    method: 'POST',
    headers: {'X-Csrf-Token': csrfToken},
    body: formData,
  });
  return await res.json();
}

function clipboardPastedImages(e) {
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


function insertAtCursor(field, value) {
  if (field.getTextArea().selectionStart || field.getTextArea().selectionStart === 0) {
    const startPos = field.getTextArea().selectionStart;
    const endPos = field.getTextArea().selectionEnd;
    field.setValue(field.getValue().substring(0, startPos) + value + field.getValue().substring(endPos, field.getValue().length));
    field.getTextArea().electionStart = startPos + value.length;
    field.getTextArea().selectionEnd = startPos + value.length;
  } else {
    field.setValue(field.getValue() + value);
  }
}

function replaceAndKeepCursor(field, oldval, newval) {
  if (field.getTextArea().selectionStart || field.getTextArea().selectionStart === 0) {
    const startPos = field.getTextArea().selectionStart;
    const endPos = field.getTextArea().selectionEnd;
    field.setValue(field.getValue().replace(oldval, newval));
    field.getTextArea().selectionStart = startPos + newval.length - oldval.length;
    field.getTextArea().selectionEnd = endPos + newval.length - oldval.length;
  } else {
    field.setValue(field.getValue().replace(oldval, newval));
  }
}

export function initCompImagePaste($target) {
  const dropzone = $target[0].querySelector('.dropzone');
  if (!dropzone) {
    return;
  }
  const uploadUrl = dropzone.getAttribute('data-upload-url');
  const dropzoneFiles = dropzone.querySelector('.files');
  $(document).on('paste', '.CodeMirror', async function (e) {
    const img = clipboardPastedImages(e.originalEvent);
    const name = img[0].name.substring(0, img[0].name.lastIndexOf('.'));
    insertAtCursor(this.CodeMirror, `![${name}]()`);
    const data = await uploadFile(img[0], uploadUrl);
    replaceAndKeepCursor(this.CodeMirror, `![${name}]()`, `![${name}](/attachments/${data.uuid})`);
    const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
    dropzoneFiles.appendChild(input[0]);
  });
}

export function initEasyMDEImagePaste(easyMDE, dropzone, files) {
  const uploadUrl = dropzone.getAttribute('data-upload-url');
  easyMDE.codemirror.on('paste', async (_, e) => {
    for (const img of clipboardPastedImages(e)) {
      const name = img.name.slice(0, img.name.lastIndexOf('.'));
      const data = await uploadFile(img, uploadUrl);
      const pos = easyMDE.codemirror.getCursor();
      easyMDE.codemirror.replaceRange(`![${name}](/attachments/${data.uuid})`, pos);
      const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
      files.append(input);
    }
  });
}
