/**
 * Attaches `input` handlers to markdown rendered tasklist checkboxes in comments.
 *
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string
 * is set accordingly and sent to the server. On success it updates the raw-content on
 * error it resets the checkbox to its original value.
 */

const preventListener = (e) => e.preventDefault();

export function initMarkupTasklist() {
  for (const el of document.querySelectorAll(`.markup[data-can-edit=true]`) || []) {
    const container = el.parentNode;
    const checkboxes = el.querySelectorAll(`.task-list-item input[type=checkbox]`);

    for (const checkbox of checkboxes) {
      if (checkbox.dataset.editable) {
        return;
      }

      checkbox.dataset.editable = 'true';
      checkbox.addEventListener('input', async () => {
        const checkboxCharacter = checkbox.checked ? 'x' : ' ';
        const position = parseInt(checkbox.dataset.sourcePosition) + 1;

        const rawContent = container.querySelector('.raw-content');
        const oldContent = rawContent.textContent;

        const encoder = new TextEncoder();
        const buffer = encoder.encode(oldContent);
        buffer.set(encoder.encode(checkboxCharacter), position);
        const newContent = new TextDecoder().decode(buffer);

        if (newContent === oldContent) {
          return;
        }

        // Prevent further inputs until the request is done. This does not use the
        // `disabled` attribute because it causes the border to flash on click.
        for (const checkbox of checkboxes) {
          checkbox.addEventListener('click', preventListener);
        }

        try {
          const editContentZone = container.querySelector('.edit-content-zone');
          const {updateUrl, context} = editContentZone.dataset;

          await $.post(updateUrl, {
            _csrf: window.config.csrf,
            content: newContent,
            context,
          });

          rawContent.textContent = newContent;
        } catch (err) {
          checkbox.checked = !checkbox.checked;
          console.error(err);
        }

        // Enable input on checkboxes again
        for (const checkbox of checkboxes) {
          checkbox.removeEventListener('click', preventListener);
        }
      });
    }

    // Enable the checkboxes as they are initially disabled by the markdown renderer
    for (const checkbox of checkboxes) {
      checkbox.disabled = false;
    }
  }
}
