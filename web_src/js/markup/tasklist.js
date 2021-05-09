/**
 * Attaches `change` handlers to markdown rendered tasklist checkboxes in comments.
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string is set accordingly and sent to the server.
 * On success it updates the raw-content on error it resets the checkbox to its original value.
 */
export default function initMarkupTasklist() {
  $(`.render-content.markup[data-can-edit='true']`).parent().each((_, container) => {
    const $container = $(container);
    const $checkboxes = $container.find(`.task-list-item input:checkbox`);

    $checkboxes.on('change', async (ev) => {
      const $checkbox = $(ev.target);
      const checkboxCharacter = $checkbox.is(':checked') ? 'x' : ' ';
      const position = parseInt($checkbox.data('source-position')) + 1;

      const $rawContent = $container.find('.raw-content');
      const oldContent = $rawContent.text();
      const newContent = oldContent.substring(0, position) + checkboxCharacter + oldContent.substring(position + 1);

      if (newContent !== oldContent) {
        $checkboxes.prop('disabled', true);

        try {
          const $contentZone = $container.find('.edit-content-zone');
          const url = $contentZone.data('update-url');
          const context = $contentZone.data('context');

          await $.post(url, {
            _csrf: window.config.csrf,
            content: newContent,
            context,
          });

          $rawContent.text(newContent);
        } catch (e) {
          $checkbox.prop('checked', !$checkbox.is(':checked'));

          console.error(e);
        } finally {
          $checkboxes.prop('disabled', false);
        }
      }
    });

    $checkboxes.prop('disabled', false);
  });
}
