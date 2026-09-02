import {generateMarkdownLinkForAttachment, decorateAttachmentPreview} from './dropzone.ts';

describe('generateMarkdownLinkForAttachment', () => {
  test('generates markdown link for attachments', () => {
    expect(generateMarkdownLinkForAttachment({name: 'test.png', uuid: 'uuid-1'})).toBe('![test.png](/attachments/uuid-1)');
    expect(generateMarkdownLinkForAttachment({name: 'retina.png', uuid: 'uuid-2'}, {width: 800, dppx: 2})).toBe('<img width="400" alt="retina.png" src="attachments/uuid-2">');
    expect(generateMarkdownLinkForAttachment({name: 'clip.mp4', uuid: 'uuid-3'})).toBe('<video src="attachments/uuid-3" title="clip.mp4" controls></video>');
    expect(generateMarkdownLinkForAttachment({name: 'archive.zip', uuid: 'uuid-4'})).toBe('[archive.zip](/attachments/uuid-4)');
  });
});

describe('decorateAttachmentPreview', () => {
  test('decorates image preview with links and copy button', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-image-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"><img data-dz-thumbnail /></div>
      <div class="dz-details">
        <div class="dz-size"><span data-dz-size>12.5 KB</span></div>
        <div class="dz-filename"><span data-dz-name>screenshot.png</span></div>
      </div>
    `;

    const file = {name: 'screenshot.png', uuid: '1111-2222-3333', previewTemplate};
    decorateAttachmentPreview(file, '/owner/repo/issues/42/attachments');

    expect(previewTemplate.getAttribute('data-tooltip-content')).toBe('/attachments/1111-2222-3333');

    const filenameLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-filename a');
    expect(filenameLink).not.toBeNull();
    expect(filenameLink!.getAttribute('href')).toBe('/owner/repo/issues/42/attachments/1111-2222-3333');
    expect(filenameLink!.getAttribute('target')).toBe('_blank');
    expect(filenameLink!.getAttribute('rel')).toBe('noreferrer');
    expect(filenameLink!.getAttribute('data-tooltip-content')).toBe('/attachments/1111-2222-3333');
    expect(filenameLink!.querySelector('[data-dz-name]')?.textContent).toBe('screenshot.png');

    const imageLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-image a');
    expect(imageLink).not.toBeNull();
    expect(imageLink!.getAttribute('href')).toBe('/owner/repo/issues/42/attachments/1111-2222-3333');
    expect(imageLink!.getAttribute('target')).toBe('_blank');
    expect(imageLink!.getAttribute('rel')).toBe('noreferrer');
    expect(imageLink!.querySelector('img')).not.toBeNull();

    const copyButton = previewTemplate.querySelector<HTMLButtonElement>('.dz-copy-link');
    expect(copyButton).not.toBeNull();
    expect(copyButton!.getAttribute('type')).toBe('button');
    expect(copyButton!.getAttribute('data-tooltip-content')).toBe('![screenshot.png](/attachments/1111-2222-3333)');
    expect(copyButton!.querySelector('.octicon-copy')).not.toBeNull();

    const event = new MouseEvent('click', {cancelable: true});
    copyButton!.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
  });

  test('handles non-image preview without thumbnail img', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-file-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"></div>
      <div class="dz-details">
        <div class="dz-size"><span data-dz-size>500 KB</span></div>
        <div class="dz-filename"><span data-dz-name>data.zip</span></div>
      </div>
    `;

    const file = {name: 'data.zip', uuid: 'zip-uuid-9999', previewTemplate};
    decorateAttachmentPreview(file, '/owner/repo/releases/attachments');

    expect(previewTemplate.getAttribute('data-tooltip-content')).toBe('/attachments/zip-uuid-9999');

    const filenameLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-filename a');
    expect(filenameLink).not.toBeNull();
    expect(filenameLink!.getAttribute('href')).toBe('/owner/repo/releases/attachments/zip-uuid-9999');
    expect(previewTemplate.querySelector('.dz-image a')).toBeNull();

    const copyButton = previewTemplate.querySelector<HTMLButtonElement>('.dz-copy-link');
    expect(copyButton).not.toBeNull();
    expect(copyButton!.getAttribute('data-tooltip-content')).toBe('[data.zip](/attachments/zip-uuid-9999)');
  });

  test('is idempotent when called multiple times on the same preview element', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-image-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"><img data-dz-thumbnail /></div>
      <div class="dz-details">
        <div class="dz-filename"><span data-dz-name>repeated.png</span></div>
      </div>
    `;

    const file = {name: 'repeated.png', uuid: 'repeat-uuid', previewTemplate};
    decorateAttachmentPreview(file, '/base/url');
    decorateAttachmentPreview(file, '/base/url');

    expect(previewTemplate.querySelectorAll('.dz-filename a').length).toBe(1);
    expect(previewTemplate.querySelectorAll('.dz-image a').length).toBe(1);
    expect(previewTemplate.querySelectorAll('.dz-copy-link').length).toBe(1);
  });
});
