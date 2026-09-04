import {svgRaw} from '../svg.ts';
import {html} from '../utils/html.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {createElementFromAttrs, hideElem, queryElems} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {isImageFile, isVideoFile} from '../utils.ts';
import type Dropzone from '@deltablot/dropzone';

type CustomDropzoneFile = {
  uuid: string;

  // the following fields are from Dropzone.DropzoneFile
  previewElement?: HTMLElement; // will be set during the "addedfile" event
  name: string;
  size: number;
};

type UploadResponse = {uuid: string};
type FileUuidDict = Record<string, {submitted: boolean}>;

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

export function generateMarkdownLinkForAttachment(file: {uuid: string, name: string}, {width, dppx}: {width?: number, dppx?: number} = {}) {
  // Markdown always renders the image with a relative path, so the final URL is "/sub-path/owner/repo/attachments/{uuid}"
  let fileMarkdown = `[${file.name}](attachments/${file.uuid})`;
  if (isImageFile({name: file.name, type: null})) {
    if (width && width > 0 && dppx && dppx > 1) {
      // Scale down images from HiDPI monitors. This uses the <img> tag because it's the only
      // method to change image size in Markdown that is supported by all implementations.
      fileMarkdown = html`<img width="${Math.round(width / dppx)}" alt="${file.name}" src="attachments/${file.uuid}">`;
    } else {
      fileMarkdown = `![${file.name}](attachments/${file.uuid})`;
    }
  } else if (isVideoFile({name: file.name, type: null})) {
    fileMarkdown = html`<video src="attachments/${file.uuid}" title="${file.name}" controls></video>`;
  }
  return fileMarkdown;
}

export function decorateAttachmentPreview(dzInst: Dropzone, file: CustomDropzoneFile, attachmentBaseLinkUrl: string) {
  const el = file.previewElement!;
  const fileUrl = `${attachmentBaseLinkUrl}/${file.uuid}`;
  queryElems<HTMLAnchorElement>(el, 'a[data-dz-custom-link]', (elLink) => {
    elLink.target = '_blank';
    elLink.href = fileUrl;
  });

  const needUuidLink = dzInst.element.getAttribute('data-need-uuid-link') === 'true';
  const elCopyLink = el.querySelector<HTMLButtonElement>('button[data-dz-custom-copy-link]')!;
  if (needUuidLink) {
    const markdownLink = generateMarkdownLinkForAttachment(file);
    el.setAttribute('data-tooltip-content', `Name: ${file.name}\nUUID: ${file.uuid}`);
    elCopyLink.setAttribute('data-clipboard-text', markdownLink);
  } else {
    hideElem(elCopyLink);
  }
}

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
    dictInvalidFileType: dropzoneEl.getAttribute('data-text-invalid-input-type')!,
    dictFileTooBig: dropzoneEl.getAttribute('data-text-file-too-big')!,
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
    // template reference: preview-template.js in the dropzone source code
    previewTemplate: html`
      <div class="dz-preview dz-file-preview">
        <div class="dz-default dz-message">
          <button class="dz-button" type="button">${dropzoneEl.getAttribute('data-text-default-message')!}</button>
        </div>
        <div class="dz-image"><a data-dz-custom-link><img data-dz-thumbnail/></a></div>
        <div class="dz-details">
          <div class="dz-size"><span data-dz-size></span></div>
          <a class="dz-filename muted" data-dz-custom-link><span data-dz-name></span></a>
        </div>
        <div class="dz-progress">
          <span class="dz-upload" data-dz-uploadprogress></span>
        </div>
        <div class="dz-error-message"><span data-dz-errormessage></span></div>
        <div class="dz-success-mark">${svgRaw('octicon-check-circle', 54, 'tw-text-green')}</div>
        <div class="dz-error-mark">${svgRaw('octicon-x-circle', 54, 'tw-text-red')}</div>
        <div class="dz-custom-buttons">
          <button type="button" class="btn" data-dz-remove>${dropzoneEl.getAttribute('data-text-remove-file')!}</button>
          <button type="button" class="btn" data-dz-custom-copy-link>${svgRaw('octicon-copy', 14)} ${dropzoneEl.getAttribute('data-text-copy-link')!}</button>
        </div>
      </div>
    `,
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
    decorateAttachmentPreview(dzInst, file, attachmentBaseLinkUrl);
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
        const file: CustomDropzoneFile = {name: attachment.name, uuid: attachment.uuid, size: attachment.size};
        dzInst.emit('addedfile', file);
        dzInst.emit('complete', file);
        if (isImageFile({name: file.name, type: null})) {
          const imgSrc = `${attachmentBaseLinkUrl}/${file.uuid}`;
          dzInst.emit('thumbnail', file, imgSrc);
        }
        decorateAttachmentPreview(dzInst, file, attachmentBaseLinkUrl); // it is from server response, so no "type"
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
