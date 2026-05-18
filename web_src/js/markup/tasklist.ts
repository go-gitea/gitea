import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

const preventListener = (e: Event) => e.preventDefault();

/**
 * Toggle a task list checkbox in markdown content.
 * `position` is the byte offset of the space or `x` character inside `[ ]`.
 * Returns the updated content, or null if the position is invalid.
 */
export function toggleTasklistCheckbox(content: string, position: number, checked: boolean): string | null {
  const buffer = new TextEncoder().encode(content);
  // Indexes may fall off the ends and return undefined.
  if (buffer[position - 1] !== '['.charCodeAt(0) ||
    buffer[position] !== ' '.charCodeAt(0) && buffer[position] !== 'x'.charCodeAt(0) ||
    buffer[position + 1] !== ']'.charCodeAt(0)) {
    return null;
  }
  buffer[position] = checked ? 'x'.charCodeAt(0) : ' '.charCodeAt(0);
  return new TextDecoder().decode(buffer);
}

/**
 * Attaches `input` handlers to markdown rendered tasklist checkboxes in comments.
 *
 * When a checkbox value changes, the corresponding [ ] or [x] in the markdown string
 * is set accordingly and sent to the server. On success, it updates the raw-content on
 * error it resets the checkbox to its original value.
 */
export function initMarkupTasklist(elMarkup: HTMLElement): void {
  if (!elMarkup.matches('[data-can-edit=true]')) return;

  const container = elMarkup.parentNode!;
  const checkboxes = elMarkup.querySelectorAll<HTMLInputElement>(`.task-list-item input[type=checkbox]`);

  for (const checkbox of checkboxes) {
    if (checkbox.hasAttribute('data-editable')) {
      return;
    }

    checkbox.setAttribute('data-editable', 'true');
    checkbox.addEventListener('input', async () => {
      const position = parseInt(checkbox.getAttribute('data-source-position')!) + 1;

      const rawContent = container.querySelector('.raw-content')!;
      const oldContent = rawContent.textContent;

      const newContent = toggleTasklistCheckbox(oldContent, position, checkbox.checked);
      if (newContent === null) {
        // Position is probably wrong. Revert and don't allow change.
        checkbox.checked = !checkbox.checked;
        throw new Error(`Expected position to be space or x and surrounded by brackets, but it's not: position=${position}`);
      }

      if (newContent === oldContent) {
        return;
      }

      // Prevent further inputs until the request is done. This does not use the
      // `disabled` attribute because it causes the border to flash on click.
      for (const checkbox of checkboxes) {
        checkbox.addEventListener('click', preventListener);
      }

      try {
        const editContentZone = container.querySelector<HTMLDivElement>('.edit-content-zone')!;
        const updateUrl = editContentZone.getAttribute('data-update-url')!;
        const context = editContentZone.getAttribute('data-context')!;
        const contentVersion = editContentZone.getAttribute('data-content-version')!;

        const requestBody = new FormData();
        requestBody.append('ignore_attachments', 'true');
        requestBody.append('content', newContent);
        requestBody.append('context', context);
        requestBody.append('content_version', contentVersion);
        const response = await POST(updateUrl, {data: requestBody});
        const data = await response.json();
        if (response.status === 400) {
          showErrorToast(data.errorMessage);
          return;
        }
        editContentZone.setAttribute('data-content-version', data.contentVersion);
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

    // Enable the checkboxes as they are initially disabled by the markdown renderer
    for (const checkbox of checkboxes) {
      checkbox.disabled = false;
    }
  }
}
