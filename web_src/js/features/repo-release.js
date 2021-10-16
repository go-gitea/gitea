import attachTribute from './tribute.js';
import {initCompMarkupContentPreviewTab} from './comp/MarkupContentPreview.js';
import {initSimpleMDEImagePaste} from './comp/ImagePaste.js';
import {createCommentSimpleMDE} from './comp/CommentSimpleMDE.js';

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

  const $textarea = $editor.find('textarea');
  attachTribute($textarea.get(), {mentions: false, emoji: true});
  const $files = $editor.parent().find('.files');
  const $simplemde = createCommentSimpleMDE($textarea);
  initCompMarkupContentPreviewTab($editor);
  const dropzone = $editor.parent().find('.dropzone')[0];
  initSimpleMDEImagePaste($simplemde, dropzone, $files);
}
