import {createToastEditor} from './toast-editor.ts';

export function initArticleEditor() {
  const editForm = document.querySelector<HTMLFormElement>('#article-edit-form');
  if (!editForm) return;

  const textarea = document.getElementById('edit_area') as HTMLTextAreaElement;
  if (!textarea) return;

  // Initialize Toast UI Editor
  (async () => {
    const editor = await createToastEditor(textarea, {
      height: '500px',
      initialEditType: 'wysiwyg',
      previewStyle: 'vertical',
      usageStatistics: false,
      hideModeSwitch: false  // Allow mode switching
    });

    // Fork button is now handled by a form in the template with proper navigation
    // No JavaScript handler needed - the form submission navigates to the fork page

    // Handle Submit Changes button
    const submitButton = document.getElementById('submit-changes-button');
    if (submitButton && !submitButton.classList.contains('disabled')) {
      submitButton.addEventListener('click', async () => {
        // Update textarea with editor content before submission
        textarea.value = editor.getMarkdown();
        
        // For now, just show a message - in production this would submit to backend
        alert(`Submit functionality would be implemented here.\n\nContent length: ${textarea.value.length} characters`);
        
        // Uncomment to actually submit:
        // editForm.submit();
      });
    }
  })();
}

