import $ from 'jquery';
import {svg} from '../svg.js';
import {htmlEscape} from 'escape-goat';
import {clippie} from 'clippie';
import {showTemporaryTooltip} from '../modules/tippy.js';
import {POST} from '../modules/fetch.js';
import {showErrorToast} from '../modules/toast.js';

const {csrfToken, i18n} = window.config;

export async function createDropzone(el, opts) {
  const [{Dropzone}] = await Promise.all([
    import(/* webpackChunkName: "dropzone" */'dropzone'),
    import(/* webpackChunkName: "dropzone" */'dropzone/dist/dropzone.css'),
  ]);
  return new Dropzone(el, opts);
}

export function initGlobalDropzone() {
  for (const el of document.querySelectorAll('.dropzone')) {
    initDropzone(el);
  }
}

export function initDropzone(el) {
  const $dropzone = $(el);
  const _promise = createDropzone(el, {
    url: $dropzone.data('upload-url'),
    headers: {'X-Csrf-Token': csrfToken},
    maxFiles: $dropzone.data('max-file'),
    maxFilesize: $dropzone.data('max-size'),
    acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
    addRemoveLinks: true,
    dictDefaultMessage: $dropzone.data('default-message'),
    dictInvalidFileType: $dropzone.data('invalid-input-type'),
    dictFileTooBig: $dropzone.data('file-too-big'),
    dictRemoveFile: $dropzone.data('remove-file'),
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
    init() {
      this.on('success', (file, data) => {
        file.uuid = data.uuid;
        const $input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
        $dropzone.find('.files').append($input);
        // Create a "Copy Link" element, to conveniently copy the image
        // or file link as Markdown to the clipboard
        const copyLinkElement = document.createElement('div');
        copyLinkElement.className = 'tw-text-center';
        // The a element has a hardcoded cursor: pointer because the default is overridden by .dropzone
        copyLinkElement.innerHTML = `<a href="#" style="cursor: pointer;">${svg('octicon-copy', 14, 'copy link')} Copy link</a>`;
        copyLinkElement.addEventListener('click', async (e) => {
          e.preventDefault();
          let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
          if (file.type.startsWith('image/')) {
            fileMarkdown = `!${fileMarkdown}`;
          } else if (file.type.startsWith('video/')) {
            fileMarkdown = `<video src="/attachments/${file.uuid}" title="${htmlEscape(file.name)}" controls></video>`;
          }
          const success = await clippie(fileMarkdown);
          showTemporaryTooltip(e.target, success ? i18n.copy_success : i18n.copy_error);
        });
        file.previewTemplate.append(copyLinkElement);
      });
      this.on('removedfile', (file) => {
        $(`#${file.uuid}`).remove();
        if ($dropzone.data('remove-url')) {
          POST($dropzone.data('remove-url'), {
            data: new URLSearchParams({file: file.uuid}),
          });
        }
      });
      this.on('error', function (file, message) {
        showErrorToast(message);
        this.removeFile(file);
      });
    },
  });
}
