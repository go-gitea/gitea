const checkboxMarkdownPattern = /\[[ x]]/g;

/**
 * Attaches `change` handlers to markdown rendered checkboxes in comments.
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string is set accordingly and sent to the server.
 * On success it updates the raw-content on error it resets the checkbox to its original value.
 */
export default function initMarkdownCheckboxes() {
  $('.comment .segment').each((_, segment) => {
    const $segment = $(segment);
    const $checkboxes = $segment.find('.render-content.markdown input:checkbox');

    const onChange = async (ev, cbIndex) => {
      const $cb = $(ev.target);
      const checkboxMarkdown = $cb.is(':checked') ? '[x]' : '[ ]';

      const $rawContent = $segment.find('.raw-content');
      const oldContent = $rawContent.text();
      const newContent = oldContent.replace(checkboxMarkdownPattern, replaceNthMatchWith(cbIndex, checkboxMarkdown));

      if (newContent !== oldContent) {
        disableAll($checkboxes);

        try {
          const url = $segment.find('.edit-content-zone').data('update-url');
          const context = $segment.find('.edit-content-zone').data('context');

          await submit(newContent, url, context);
          $rawContent.text(newContent);
        } catch (e) {
          $cb.prop('checked', !$cb.is(':checked'));

          console.error(e);
        } finally {
          enableAll($checkboxes);
        }
      }
    };

    enableAll($checkboxes);
    $checkboxes.each((cbIndex, cb) => {
      $(cb).on('change', (ev) => onChange(ev, cbIndex));
    });
  });
}

function enableAll ($checkboxes) { $checkboxes.removeAttr('disabled') }
function disableAll ($checkboxes) { $checkboxes.attr('disabled', 'disabled') }

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
