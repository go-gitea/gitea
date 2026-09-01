import {generateMarkdownLinkForAttachment, decorateAttachmentPreview} from './dropzone.ts';

describe('generateMarkdownLinkForAttachment', () => {
  test('generates markdown for various image formats', () => {
    expect(generateMarkdownLinkForAttachment({name: 'test.png', uuid: 'uuid-1'})).toBe('![test.png](/attachments/uuid-1)');
    expect(generateMarkdownLinkForAttachment({name: 'photo.JPG', uuid: 'uuid-2'})).toBe('![photo.JPG](/attachments/uuid-2)');
    expect(generateMarkdownLinkForAttachment({name: 'vector.svg', uuid: 'uuid-3'})).toBe('![vector.svg](/attachments/uuid-3)');
    expect(generateMarkdownLinkForAttachment({name: 'animated.gif', uuid: 'uuid-4'})).toBe('![animated.gif](/attachments/uuid-4)');
    expect(generateMarkdownLinkForAttachment({name: 'modern.webp', uuid: 'uuid-5'})).toBe('![modern.webp](/attachments/uuid-5)');
    expect(generateMarkdownLinkForAttachment({name: 'photo.heic', uuid: 'uuid-6'})).toBe('![photo.heic](/attachments/uuid-6)');
    expect(generateMarkdownLinkForAttachment({name: 'photo.avif', uuid: 'uuid-7'})).toBe('![photo.avif](/attachments/uuid-7)');
    expect(generateMarkdownLinkForAttachment({name: 'noext', type: 'image/png', uuid: 'uuid-8'})).toBe('![noext](/attachments/uuid-8)');
  });

  test('scales down HiDPI images when width and dppx are provided', () => {
    expect(generateMarkdownLinkForAttachment({name: 'retina.png', uuid: 'uuid-9'}, {width: 800, dppx: 2})).toBe('<img width="400" alt="retina.png" src="attachments/uuid-9">');
    expect(generateMarkdownLinkForAttachment({name: 'retina3x.png', uuid: 'uuid-10'}, {width: 900, dppx: 3})).toBe('<img width="300" alt="retina3x.png" src="attachments/uuid-10">');
    // should not scale if dppx <= 1 or width <= 0
    expect(generateMarkdownLinkForAttachment({name: 'standard.png', uuid: 'uuid-11'}, {width: 800, dppx: 1})).toBe('![standard.png](/attachments/uuid-11)');
    expect(generateMarkdownLinkForAttachment({name: 'standard.png', uuid: 'uuid-12'}, {width: 0, dppx: 2})).toBe('![standard.png](/attachments/uuid-12)');
  });

  test('generates video tag for video formats', () => {
    expect(generateMarkdownLinkForAttachment({name: 'clip.mp4', uuid: 'uuid-13'})).toBe('<video src="attachments/uuid-13" title="clip.mp4" controls></video>');
    expect(generateMarkdownLinkForAttachment({name: 'video.webm', uuid: 'uuid-14'})).toBe('<video src="attachments/uuid-14" title="video.webm" controls></video>');
    expect(generateMarkdownLinkForAttachment({name: 'movie.mkv', uuid: 'uuid-15'})).toBe('<video src="attachments/uuid-15" title="movie.mkv" controls></video>');
    expect(generateMarkdownLinkForAttachment({name: 'record.mpeg', uuid: 'uuid-16'})).toBe('<video src="attachments/uuid-16" title="record.mpeg" controls></video>');
    expect(generateMarkdownLinkForAttachment({name: 'stream', type: 'video/mp4', uuid: 'uuid-17'})).toBe('<video src="attachments/uuid-17" title="stream" controls></video>');
  });

  test('generates standard link for non-image / non-video files and special characters', () => {
    expect(generateMarkdownLinkForAttachment({name: 'archive.zip', uuid: 'uuid-18'})).toBe('[archive.zip](/attachments/uuid-18)');
    expect(generateMarkdownLinkForAttachment({name: 'document.pdf', uuid: 'uuid-19'})).toBe('[document.pdf](/attachments/uuid-19)');
    expect(generateMarkdownLinkForAttachment({name: 'report (2026) [draft].tar.gz', uuid: 'uuid-20'})).toBe('[report (2026) [draft].tar.gz](/attachments/uuid-20)');
    expect(generateMarkdownLinkForAttachment({name: 'my file with spaces.txt', uuid: 'uuid-21'})).toBe('[my file with spaces.txt](/attachments/uuid-21)');
    expect(generateMarkdownLinkForAttachment({name: 'code.go', uuid: 'uuid-22'})).toBe('[code.go](/attachments/uuid-22)');
  });
});

describe('decorateAttachmentPreview', () => {
  test('decorates image preview with custom attachmentBaseLinkUrl', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-image-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"><img data-dz-thumbnail /></div>
      <div class="dz-details">
        <div class="dz-size"><span data-dz-size>12.5 KB</span></div>
        <div class="dz-filename"><span data-dz-name>screenshot.png</span></div>
      </div>
    `;

    decorateAttachmentPreview(
      {name: 'screenshot.png', uuid: '1111-2222-3333', previewTemplate},
      '/owner/repo/issues/42/attachments',
    );

    // Root preview tooltip
    expect(previewTemplate.getAttribute('data-tooltip-content')).toBe('/attachments/1111-2222-3333');

    // Filename link
    const filenameLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-filename a');
    expect(filenameLink).not.toBeNull();
    expect(filenameLink!.getAttribute('href')).toBe('/owner/repo/issues/42/attachments/1111-2222-3333');
    expect(filenameLink!.getAttribute('target')).toBe('_blank');
    expect(filenameLink!.getAttribute('rel')).toBe('noreferrer');
    expect(filenameLink!.getAttribute('data-tooltip-content')).toBe('/attachments/1111-2222-3333');
    expect(filenameLink!.querySelector('[data-dz-name]')?.textContent).toBe('screenshot.png');

    // Image thumbnail link
    const imageLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-image a');
    expect(imageLink).not.toBeNull();
    expect(imageLink!.getAttribute('href')).toBe('/owner/repo/issues/42/attachments/1111-2222-3333');
    expect(imageLink!.getAttribute('target')).toBe('_blank');
    expect(imageLink!.getAttribute('rel')).toBe('noreferrer');
    expect(imageLink!.querySelector('img')).not.toBeNull();

    // Copy link button
    const copyButton = previewTemplate.querySelector<HTMLButtonElement>('.dz-copy-link');
    expect(copyButton).not.toBeNull();
    expect(copyButton!.getAttribute('type')).toBe('button');
    expect(copyButton!.getAttribute('data-tooltip-content')).toBe('![screenshot.png](/attachments/1111-2222-3333)');
    expect(copyButton!.querySelector('.octicon-copy')).not.toBeNull();
    expect(copyButton!.textContent?.trim()).toContain('Copy link');
  });

  test('falls back to /attachments/{uuid} when attachmentBaseLinkUrl is omitted', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-image-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"><img data-dz-thumbnail /></div>
      <div class="dz-details">
        <div class="dz-filename"><span data-dz-name>image.png</span></div>
      </div>
    `;

    decorateAttachmentPreview({name: 'image.png', uuid: 'default-url-uuid', previewTemplate});

    const filenameLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-filename a');
    expect(filenameLink!.getAttribute('href')).toBe('/attachments/default-url-uuid');

    const imageLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-image a');
    expect(imageLink!.getAttribute('href')).toBe('/attachments/default-url-uuid');
  });

  test('handles non-image preview without thumbnail img gracefully', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.className = 'dz-preview dz-file-preview';
    previewTemplate.innerHTML = `
      <div class="dz-image"></div>
      <div class="dz-details">
        <div class="dz-size"><span data-dz-size>500 KB</span></div>
        <div class="dz-filename"><span data-dz-name>data.zip</span></div>
      </div>
    `;

    decorateAttachmentPreview(
      {name: 'data.zip', uuid: 'zip-uuid-9999', previewTemplate},
      '/owner/repo/releases/attachments',
    );

    expect(previewTemplate.getAttribute('data-tooltip-content')).toBe('/attachments/zip-uuid-9999');

    const filenameLink = previewTemplate.querySelector<HTMLAnchorElement>('.dz-filename a');
    expect(filenameLink).not.toBeNull();
    expect(filenameLink!.getAttribute('href')).toBe('/owner/repo/releases/attachments/zip-uuid-9999');

    // No img element, so no image link created
    expect(previewTemplate.querySelector('.dz-image a')).toBeNull();

    // Copy link button generated for non-image markdown link
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
    decorateAttachmentPreview(file, '/base/url');

    // Ensure only single link wrapper exists
    expect(previewTemplate.querySelectorAll('.dz-filename a').length).toBe(1);
    expect(previewTemplate.querySelectorAll('.dz-image a').length).toBe(1);
    expect(previewTemplate.querySelectorAll('.dz-copy-link').length).toBe(1);
  });

  test('handles missing previewTemplate or missing uuid safely without throwing', () => {
    expect(() => decorateAttachmentPreview({})).not.toThrow();
    expect(() => decorateAttachmentPreview({uuid: 'test-uuid'})).not.toThrow();
    expect(() => decorateAttachmentPreview({previewTemplate: document.createElement('div')})).not.toThrow();
  });

  test('copy button is created with correct tooltip and type', () => {
    const previewTemplate = document.createElement('div');
    previewTemplate.innerHTML = `
      <div class="dz-details">
        <div class="dz-filename"><span data-dz-name>test.png</span></div>
      </div>
    `;

    decorateAttachmentPreview({name: 'test.png', uuid: 'click-uuid', previewTemplate});

    const copyButton = previewTemplate.querySelector<HTMLButtonElement>('.dz-copy-link');
    expect(copyButton).not.toBeNull();
    expect(copyButton!.getAttribute('type')).toBe('button');
    expect(copyButton!.getAttribute('data-tooltip-content')).toBe('![test.png](/attachments/click-uuid)');
    expect(copyButton!.querySelector('.octicon-copy')).not.toBeNull();

    const event = new MouseEvent('click', {cancelable: true});
    copyButton!.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
  });
});
