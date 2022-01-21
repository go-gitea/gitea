import attachTribute from './tribute.js';
import {initCompMarkupContentPreviewTab} from './comp/MarkupContentPreview.js';
import {initEasyMDEImagePaste} from './comp/ImagePaste.js';
import {createCommentEasyMDE} from './comp/EasyMDE.js';

export function initRepoRelease() {
  $(document).on('click', '.remove-rel-attach', function() {
    const uuid = $(this).data('uuid');
    const id = $(this).data('id');
    $(`input[name='attachment-del-${uuid}']`).attr('value', true);
    $(`#attachment-${id}`).hide();
  });
}


export function initRepoReleaseEditor() {
  const $editor = $('.repository.new.release .content-editor');
  if ($editor.length === 0) {
    return false;
  }

  (async () => {
    const $textarea = $editor.find('textarea');
    await attachTribute($textarea.get(), {mentions: false, emoji: true});
    const $files = $editor.parent().find('.files');
    const easyMDE = await createCommentEasyMDE($textarea);
    initCompMarkupContentPreviewTab($editor);
    const dropzone = $editor.parent().find('.dropzone')[0];
    initEasyMDEImagePaste(easyMDE, dropzone, $files);
  })();
}
