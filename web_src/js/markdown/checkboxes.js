/**
 * Affects first comment of issue page only!
 *
 * Attaches `change` handlers to the markdown rendered checkboxes.
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string is set accordingly and sent to the server.
 * On success it updates the raw-content on error it resets the checkbox to its original value.
 */
export default function initMarkdownCheckboxes() {
  const $segment = $('.page-content.issue .comment.first .segment');
  const $checkboxes = $segment.find('.render-content.markdown input:checkbox');
  const $rawContent = $segment.find('.raw-content');

  const url = $segment.find('.edit-content-zone').data('update-url');
  const context = $segment.find('.edit-content-zone').data('context');

  const checkboxMarkdownPattern = /\[[ x]]/g;

  const enableAll = () => $checkboxes.removeAttr('disabled');
  const disableAll = () => $checkboxes.attr('disabled', 'disabled');

  const onChange = async (ev, cbIndex) => {
    const $cb = $(ev.target);
    const checkboxMarkdown = $cb.is(':checked') ? '[x]' : '[ ]';

    const oldContent = $rawContent.text();
    const newContent = oldContent.replace(checkboxMarkdownPattern, replaceNthMatchWith(cbIndex, checkboxMarkdown));

    if (newContent !== oldContent) {
      disableAll();

      try {
        await submit(newContent, url, context);
        $rawContent.text(newContent);
      } catch (e) {
        $cb.prop('checked', !$cb.is(':checked'));

        console.error(e);
      } finally {
        enableAll();
      }
    }
  };

  enableAll();
  $checkboxes.each((cbIndex, cb) => {
    $(cb).on('change', (ev) => onChange(ev, cbIndex));
  });
}

function submit (content, url, context) {
  const csrf = window.config.csrf;

  return $.post(url, {
    _csrf: csrf,
    context,
    content,
  });
}


function replaceNthMatchWith(n, replaceWith) {
  let matchIndex = 0;

  return (match) => {
    if (n === matchIndex++) {
      return replaceWith;
    }

    return match;
  };
}
