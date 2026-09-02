import {svgRaw} from '../svg.ts';
import {html} from '../utils/html.ts';
import {copyToClipboardWithFeedback} from '../modules/clipboard.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {createElementFromHTML, createElementFromAttrs} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {isImageFile, isVideoFile} from '../utils.ts';
import type Dropzone from '@deltablot/dropzone';

type CustomDropzoneFile = Dropzone.DropzoneFile & {uuid: string};
type UploadResponse = {uuid: string};

// dropzone has its owner event dispatcher (emitter)
export const DropzoneCustomEventReloadFiles = 'dropzone-custom-reload-files';
export const DropzoneCustomEventRemovedFile = 'dropzone-custom-removed-file';
export const DropzoneCustomEventUploadDone = 'dropzone-custom-upload-done';

async function createDropzone(el: HTMLElement, opts: Dropzone.DropzoneOptions) {
  const [{default: Dropzone}] = await Promise.all([
    import('@deltablot/dropzone'),
    import('@deltablot/dropzone/dist/dropzone.css'),
  ]);
  return new Dropzone(el, opts);
}

export function generateMarkdownLinkForAttachment(file: Partial<CustomDropzoneFile>, {width, dppx}: {width?: number, dppx?: number} = {}) {
  let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
  if (isImageFile(file)) {
    if (width && width > 0 && dppx && dppx > 1) {
      // Scale down images from HiDPI monitors. This uses the <img> tag because it's the only
      // method to change image size in Markdown that is supported by all implementations.
      // Make the image link relative to the repo path, then the final URL is "/sub-path/owner/repo/attachments/{uuid}"
      fileMarkdown = html`<img width="${Math.round(width / dppx)}" alt="${file.name}" src="attachments/${file.uuid}">`;
    } else {
      // Markdown always renders the image with a relative path, so the final URL is "/sub-path/owner/repo/attachments/{uuid}"
      // TODO: it should also use relative path for consistency, because absolute is ambiguous for "/sub-path/attachments" or "/attachments"
      fileMarkdown = `![${file.name}](/attachments/${file.uuid})`;
    }
  } else if (isVideoFile(file)) {
    fileMarkdown = html`<video src="attachments/${file.uuid}" title="${file.name}" controls></video>`;
  }
  return fileMarkdown;
}

export function decorateAttachmentPreview(file: Partial<CustomDropzoneFile>, attachmentBaseLinkUrl: string) {
  // previewTemplate always exists, but tsc doesnt know about it
  const el = file.previewTemplate!;

  const fileUrl = `${attachmentBaseLinkUrl}/${file.uuid}`;
  const markdownLink = generateMarkdownLinkForAttachment(file);

  el.setAttribute('data-tooltip-content', `/attachments/${file.uuid}`);

  const nameSpan = el.querySelector<HTMLElement>('[data-dz-name]');
  if (nameSpan && !nameSpan.closest('a')) {
    const linkEl = document.createElement('a');
    linkEl.href = fileUrl;
    linkEl.target = '_blank';
    linkEl.rel = 'noreferrer';
    linkEl.className = 'tw-text-inherit hover:tw-underline';
    linkEl.setAttribute('data-tooltip-content', `/attachments/${file.uuid}`);
    nameSpan.replaceWith(linkEl);
    linkEl.append(nameSpan);
  }

  const imgEl = el.querySelector<HTMLImageElement>('.dz-image img');
  if (imgEl && !imgEl.closest('a')) {
    const linkEl = document.createElement('a');
    linkEl.href = fileUrl;
    linkEl.target = '_blank';
    linkEl.rel = 'noreferrer';
    imgEl.replaceWith(linkEl);
    linkEl.append(imgEl);
  }

  if (!el.querySelector('.dz-copy-link')) {
    const copyButton = createElementFromHTML<HTMLButtonElement>(html`
      <button type="button" class="dz-copy-link tw-block tw-w-full tw-text-center tw-mt-1 tw-bg-transparent tw-p-0 tw-text-text-light hover:tw-text-text" data-tooltip-content="${markdownLink}">
        ${svgRaw('octicon-copy', 14)} Copy link
      </button>
    `);
    copyButton.addEventListener('click', async (e) => {
      e.preventDefault();
      await copyToClipboardWithFeedback(copyButton, markdownLink);
    });
    el.append(copyButton);
  }
}

type FileUuidDict = Record<string, {submitted: boolean}>;

/**
 * @param {HTMLElement} dropzoneEl
 */
export async function initDropzone(dropzoneEl: HTMLElement) {
  const listAttachmentsUrl = dropzoneEl.closest('[data-attachment-url]')?.getAttribute('data-attachment-url');
  const removeAttachmentUrl = dropzoneEl.getAttribute('data-remove-url')!;
  const attachmentBaseLinkUrl = dropzoneEl.getAttribute('data-link-url')!;

  let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
  let fileUuidDict: FileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone
  const opts: Dropzone.DropzoneOptions = {
    url: dropzoneEl.getAttribute('data-upload-url')!,
    addRemoveLinks: true,
    dictDefaultMessage: dropzoneEl.getAttribute('data-default-message')!,
    dictInvalidFileType: dropzoneEl.getAttribute('data-invalid-input-type')!,
    dictFileTooBig: dropzoneEl.getAttribute('data-file-too-big')!,
    dictRemoveFile: dropzoneEl.getAttribute('data-remove-file')!,
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
  };
  const accepts = dropzoneEl.getAttribute('data-accepts')!;
  if (!['*/*', ''].includes(accepts)) opts.acceptedFiles = accepts;
  if (dropzoneEl.hasAttribute('data-max-file')) opts.maxFiles = Number(dropzoneEl.getAttribute('data-max-file'));
  if (dropzoneEl.hasAttribute('data-max-size')) opts.maxFilesize = Number(dropzoneEl.getAttribute('data-max-size'));

  // there is a bug in dropzone: if a non-image file is uploaded, then it tries to request the file from server by something like:
  // "http://localhost:3000/owner/repo/issues/[object%20Event]"
  // the reason is that the preview "callback(dataURL)" is assign to "img.onerror" then "thumbnail" uses the error object as the dataURL and generates '<img src="[object Event]">'
  const dzInst = await createDropzone(dropzoneEl, opts);
  dzInst.on('success', (file: CustomDropzoneFile, resp: UploadResponse) => {
    file.uuid = resp.uuid;
    fileUuidDict[file.uuid] = {submitted: false};
    const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${resp.uuid}`, value: resp.uuid});
    dropzoneEl.querySelector('.files')!.append(input);
    decorateAttachmentPreview(file, attachmentBaseLinkUrl);
    dzInst.emit(DropzoneCustomEventUploadDone, {file});
  });

  dzInst.on('removedfile', async (file: CustomDropzoneFile) => {
    if (disableRemovedfileEvent) return;

    dzInst.emit(DropzoneCustomEventRemovedFile, {fileUuid: file.uuid});
    document.querySelector(`#dropzone-file-${file.uuid}`)?.remove();
    // when the uploaded file number reaches the limit, there is no uuid in the dict, and it doesn't need to be removed from server
    if (removeAttachmentUrl && fileUuidDict[file.uuid] && !fileUuidDict[file.uuid].submitted) {
      await POST(removeAttachmentUrl, {data: new URLSearchParams({file: file.uuid})});
    }
  });

  dzInst.on('submit', () => {
    for (const value of Object.values(fileUuidDict)) {
      value.submitted = true;
    }
  });

  dzInst.on(DropzoneCustomEventReloadFiles, async () => {
    try {
      if (!listAttachmentsUrl) return;
      const resp = await GET(listAttachmentsUrl);
      const respData = await resp.json();
      // do not trigger the "removedfile" event, otherwise the attachments would be deleted from server
      disableRemovedfileEvent = true;
      dzInst.removeAllFiles(true);
      disableRemovedfileEvent = false;

      dropzoneEl.querySelector('.files')!.replaceChildren();
      for (const el of dropzoneEl.querySelectorAll('.dz-preview')) el.remove();
      fileUuidDict = {};
      for (const attachment of respData) {
        const file = {name: attachment.name, uuid: attachment.uuid, size: attachment.size};
        dzInst.emit('addedfile', file);
        dzInst.emit('complete', file);
        if (isImageFile(file)) {
          const imgSrc = `${attachmentBaseLinkUrl}/${file.uuid}`;
          dzInst.emit('thumbnail', file, imgSrc);
        }
        decorateAttachmentPreview(file, attachmentBaseLinkUrl); // it is from server response, so no "type"
        fileUuidDict[file.uuid] = {submitted: true};
        const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${file.uuid}`, value: file.uuid});
        dropzoneEl.querySelector('.files')!.append(input);
      }
      if (!dropzoneEl.querySelector('.dz-preview')) {
        dropzoneEl.classList.remove('dz-started');
      }
    } catch (error) {
      // TODO: if listing the existing attachments failed, it should stop from operating the content or attachments,
      //  otherwise the attachments might be lost.
      showErrorToast(`Failed to load attachments: ${errorMessage(error)}`);
      console.error(error);
    }
  });

  dzInst.on('error', (file, message) => {
    showErrorToast(`Dropzone upload error: ${message}`);
    dzInst.removeFile(file);
  });

  if (listAttachmentsUrl) dzInst.emit(DropzoneCustomEventReloadFiles);
  return dzInst;
}
