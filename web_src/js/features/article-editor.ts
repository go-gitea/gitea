import {createToastEditor} from './toast-editor.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initArticleEditor() {
  const editForm = document.querySelector<HTMLFormElement>('#article-edit-form');
  if (!editForm) return;

  const textarea = document.getElementById('edit_area') as HTMLTextAreaElement;
  if (!textarea) return;

  // Initialize Fomantic tabs
  const elTabMenu = editForm.querySelector('.repo-editor-menu');
  if (elTabMenu) {
    fomanticQuery(elTabMenu.querySelectorAll('.item')).tab();
  }

  // Initialize Toast UI Editor
  (async () => {
    const editor = await createToastEditor(textarea, {
      height: '500px',
      initialEditType: 'wysiwyg',
      previewStyle: 'vertical',
      usageStatistics: false,
      hideModeSwitch: true
    });

    // Handle preview tab
    const previewTab = editForm.querySelector<HTMLAnchorElement>('a[data-tab="preview"]');
    const previewPanel = editForm.querySelector<HTMLElement>('.tab[data-tab="preview"]');
    if (previewTab && previewPanel) {
      previewTab.addEventListener('click', async () => {
        const previewUrl = previewTab.getAttribute('data-preview-url');
        const previewContextRef = previewTab.getAttribute('data-preview-context-ref');
        const readmeTreePath = previewTab.getAttribute('data-readme-path');
        
        const formData = new FormData();
        formData.append('mode', 'file');
        formData.append('context', `${previewContextRef}/${readmeTreePath}`);
        formData.append('text', editor.getMarkdown());
        formData.append('file_path', readmeTreePath || 'README.md');
        
        try {
          const response = await fetch(previewUrl, {
            method: 'POST',
            body: formData
          });
          const data = await response.text();
          previewPanel.innerHTML = `<div class="render-content markup">${data}</div>`;
        } catch (error) {
          console.error('Preview error:', error);
          previewPanel.innerHTML = '<div class="ui error message">Failed to load preview</div>';
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

