// web_src/js/features/repo-tree-search.ts

// Feature 1: File Search Functionality
export function initRepoTreeSearch() {
  const searchInput = document.querySelector<HTMLInputElement>('#repo-files-search');
  if (!searchInput) return;

  console.log('✅ Repository file search initialized');

  let searchTimeout: number;

  searchInput.addEventListener('input', (e) => {
    clearTimeout(searchTimeout);
    
    searchTimeout = window.setTimeout(() => {
      const query = (e.target as HTMLInputElement).value.toLowerCase().trim();
      filterFiles(query);
    }, 200);
  });

  searchInput.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      searchInput.value = '';
      filterFiles('');
      searchInput.blur();
    }
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === '/' && document.activeElement !== searchInput) {
      e.preventDefault();
      searchInput.focus();
    }
  });
}

function filterFiles(query: string) {
  const fileItems = document.querySelectorAll<HTMLElement>('.repo-file-item');
  
  if (fileItems.length === 0) {
    console.warn('⚠️ No file items found');
    return;
  }

  let visibleCount = 0;

  fileItems.forEach((item) => {
    const nameElement = item.querySelector<HTMLElement>('.entry-name');
    if (!nameElement) return;

    const filename = nameElement.textContent?.trim().toLowerCase() || '';

    if (query === '' || filename.includes(query)) {
      item.style.display = '';
      visibleCount++;
      
      if (query !== '') {
        highlightMatch(nameElement, query);
      } else {
        removeHighlight(nameElement);
      }
    } else {
      item.style.display = 'none';
    }
  });

  console.log(`🔍 Found ${visibleCount} matching file(s)`);
  showNoResults(visibleCount, query);
}

function highlightMatch(element: HTMLElement, query: string) {
  if (!element.dataset.originalHtml) {
    element.dataset.originalHtml = element.innerHTML;
  }

  const originalText = element.textContent?.trim() || '';
  const lowerText = originalText.toLowerCase();
  const index = lowerText.indexOf(query);

  if (index !== -1) {
    const before = originalText.substring(0, index);
    const match = originalText.substring(index, index + query.length);
    const after = originalText.substring(index + query.length);

    element.innerHTML = `${before}<mark style="background-color: #ffeb3b; padding: 2px 4px; border-radius: 3px; font-weight: 500;">${match}</mark>${after}`;
  }
}

function removeHighlight(element: HTMLElement) {
  if (element.dataset.originalHtml) {
    element.innerHTML = element.dataset.originalHtml;
  }
}

function showNoResults(count: number, query: string) {
  const filesTable = document.getElementById('repo-files-table');
  if (!filesTable) return;

  const existingMessage = filesTable.querySelector('.no-results-message');
  if (existingMessage) {
    existingMessage.remove();
  }

  if (count === 0 && query !== '') {
    const message = document.createElement('div');
    message.className = 'no-results-message';
    message.style.cssText = 'padding: 40px; text-align: center; color: #888;';
    message.innerHTML = `
      <div style="font-size: 48px; margin-bottom: 10px;">🔍</div>
      <div style="font-size: 16px; font-weight: 500;">No files found matching "${query}"</div>
      <div style="font-size: 14px; margin-top: 5px;">Try a different search term</div>
    `;
    filesTable.appendChild(message);
  }
}

// Feature 2 & 3: Directory Actions (Add File & Context Menu)
export function initRepoDirectoryActions() {
  console.log('✅ Repository directory actions initialized');
  
  // Initialize dropdowns
  initAddFileDropdown();
  initContextMenuDropdown();
  
  // Initialize button actions
  initCopyPath();
  initCopyPermalink();
  initDeleteDirectory();
  initCenterContentToggle();
}

function initAddFileDropdown() {
  const dropdown = document.querySelector('#add-file-dropdown');
  if (dropdown && (window as any).$ && (window as any).$.fn.dropdown) {
    (window as any).$(dropdown).dropdown();
    console.log('✅ Add file dropdown initialized');
  }
}

function initContextMenuDropdown() {
  const dropdown = document.querySelector('#directory-options-dropdown');
  if (dropdown && (window as any).$ && (window as any).$.fn.dropdown) {
    (window as any).$(dropdown).dropdown();
    console.log('✅ Context menu dropdown initialized');
  }
}

function initCopyPath() {
  const button = document.querySelector('#copy-path-button');
  if (!button) return;

  button.addEventListener('click', async (e) => {
    e.preventDefault();
    const path = (button as HTMLElement).dataset.path || '';
    
    try {
      await navigator.clipboard.writeText(path);
      showToast('✅ Path copied to clipboard!');
      console.log('Copied path:', path);
    } catch (err) {
      console.error('Failed to copy path:', err);
      showToast('❌ Failed to copy path');
    }
  });
}

function initCopyPermalink() {
  const button = document.querySelector('#copy-permalink-button');
  if (!button) return;

  button.addEventListener('click', async (e) => {
    e.preventDefault();
    const permalink = (button as HTMLElement).dataset.permalink || '';
    const fullUrl = window.location.origin + permalink;
    
    try {
      await navigator.clipboard.writeText(fullUrl);
      showToast('✅ Permalink copied to clipboard!');
      console.log('Copied permalink:', fullUrl);
    } catch (err) {
      console.error('Failed to copy permalink:', err);
      showToast('❌ Failed to copy permalink');
    }
  });
}

function initDeleteDirectory() {
  const button = document.querySelector('#delete-directory-button');
  if (!button) return;

  button.addEventListener('click', (e) => {
    e.preventDefault();
    const path = (button as HTMLElement).dataset.path || '';
    
    if (confirm(`Are you sure you want to delete the directory "${path}"?`)) {
      // This would need backend support - for now just log
      console.log('Delete directory requested:', path);
      showToast('⚠️ Delete functionality requires backend implementation');
      // TODO: Implement actual delete API call
    }
  });
}

function initCenterContentToggle() {
  const button = document.querySelector('#center-content-toggle');
  if (!button) return;

  button.addEventListener('click', (e) => {
    e.preventDefault();
    const container = document.querySelector('.ui.container');
    
    if (container) {
      container.classList.toggle('centered-content');
      const isCentered = container.classList.contains('centered-content');
      showToast(isCentered ? '✅ Content centered' : '✅ Content un-centered');
      console.log('Center content toggled:', isCentered);
    }
  });
}

function showToast(message: string) {
  // Simple toast notification
  const toast = document.createElement('div');
  toast.textContent = message;
  toast.style.cssText = `
    position: fixed;
    top: 20px;
    right: 20px;
    background: #333;
    color: white;
    padding: 12px 20px;
    border-radius: 4px;
    z-index: 10000;
    box-shadow: 0 2px 8px rgba(0,0,0,0.3);
  `;
  document.body.appendChild(toast);
  
  setTimeout(() => {
    toast.remove();
  }, 3000);
}
