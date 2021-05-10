/**
 * Attaches `change` handlers to markdown rendered tasklist checkboxes in comments.
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string is set accordingly and sent to the server.
 * On success it updates the raw-content on error it resets the checkbox to its original value.
 */
export default function initMarkupTasklist() {
  document.querySelectorAll(`.render-content.markup[data-can-edit='true']`).forEach((el) => {
    const container = el.parentNode;
    const checkboxes = container.querySelectorAll(`.task-list-item input[type=checkbox]`);

    checkboxes.forEach((cb) => cb.addEventListener('change', async (ev) => {
      const checkbox = ev.target;
      const checkboxCharacter = checkbox.checked ? 'x' : ' ';
      const position = parseInt(checkbox.dataset.sourcePosition) + 1;

      const rawContent = container.querySelector('.raw-content');
      const oldContent = rawContent.textContent;
      const newContent = oldContent.substring(0, position) + checkboxCharacter + oldContent.substring(position + 1);

      if (newContent !== oldContent) {
        checkboxes.forEach((cb) => cb.disabled = true);

        try {
          const contentZone = container.querySelector('.edit-content-zone');
          const url = contentZone.dataset.updateUrl;
          const context = contentZone.dataset.context;

          await $.post(url, {
            _csrf: window.config.csrf,
            content: newContent,
            context,
          });

          rawContent.textContent = newContent;
        } catch (e) {
          checkbox.checked = !checkbox.checked;

          console.error(e);
        } finally {
          checkboxes.forEach((cb) => cb.disabled = false);
        }
      }
    }));

    checkboxes.forEach((cb) => cb.disabled = false);
  });
}
