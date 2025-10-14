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

    // Handle switching between visual (WYSIWYG) and source (Markdown) modes
    const editTab = editForm.querySelector<HTMLAnchorElement>('a[data-tab="write"]');
    const sourceTab = editForm.querySelector<HTMLAnchorElement>('a[data-tab="preview"]');
    
    if (editTab && sourceTab) {
      // Set initial state - visual mode
      let isVisualMode = true;

      editTab.addEventListener('click', (e) => {
        e.preventDefault();
        if (!isVisualMode) {
          // Switch to visual mode
          editor.changeMode('wysiwyg');
          isVisualMode = true;
          editTab.classList.add('active');
          sourceTab.classList.remove('active');
        }
      });

      sourceTab.addEventListener('click', (e) => {
        e.preventDefault();
        if (isVisualMode) {
          // Switch to source mode
          editor.changeMode('markdown');
          isVisualMode = false;
          sourceTab.classList.add('active');
          editTab.classList.remove('active');
        }
      });
    }

    // Handle Fork button
    const forkButton = document.getElementById('fork-button');
    if (forkButton && !forkButton.classList.contains('disabled')) {
      forkButton.addEventListener('click', async () => {
        const repoLink = forkButton.getAttribute('data-repo-link');
        if (repoLink && confirm('Do you want to fork this repository?')) {
          window.location.href = `${repoLink}/fork`;
        }
      });
    }

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

