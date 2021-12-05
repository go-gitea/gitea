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
  if (field.selectionStart || field.selectionStart === 0) {
    const startPos = field.selectionStart;
    const endPos = field.selectionEnd;
    field.value = field.value.substring(0, startPos) + value + field.value.substring(endPos, field.value.length);
    field.selectionStart = startPos + value.length;
    field.selectionEnd = startPos + value.length;
  } else {
    field.value += value;
  }
}

function replaceAndKeepCursor(field, oldval, newval) {
  if (field.selectionStart || field.selectionStart === 0) {
    const startPos = field.selectionStart;
    const endPos = field.selectionEnd;
    field.value = field.value.replace(oldval, newval);
    field.selectionStart = startPos + newval.length - oldval.length;
    field.selectionEnd = endPos + newval.length - oldval.length;
  } else {
    field.value = field.value.replace(oldval, newval);
  }
}

export function initCompImagePaste($target) {
  $target.each(function () {
    const dropzone = this.querySelector('.dropzone');
    if (!dropzone) {
      return;
    }
    const uploadUrl = dropzone.getAttribute('data-upload-url');
    const dropzoneFiles = dropzone.querySelector('.files');
    for (const textarea of this.querySelectorAll('textarea')) {
      textarea.addEventListener('paste', async (e) => {
        for (const img of clipboardPastedImages(e)) {
          const name = img.name.substr(0, img.name.lastIndexOf('.'));
          insertAtCursor(textarea, `![${name}]()`);
          const data = await uploadFile(img, uploadUrl);
          replaceAndKeepCursor(textarea, `![${name}]()`, `![${name}](/attachments/${data.uuid})`);
          const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
          dropzoneFiles.appendChild(input[0]);
        }
      }, false);
    }
  });
}

export function initSimpleMDEImagePaste(simplemde, dropzone, files) {
  const uploadUrl = dropzone.getAttribute('data-upload-url');
  simplemde.codemirror.on('paste', async (_, e) => {
    for (const img of clipboardPastedImages(e)) {
      const name = img.name.substr(0, img.name.lastIndexOf('.'));
      const data = await uploadFile(img, uploadUrl);
      const pos = simplemde.codemirror.getCursor();
      simplemde.codemirror.replaceRange(`![${name}](/attachments/${data.uuid})`, pos);
      const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
      files.append(input);
    }
  });
}
