// RepoFileSearch Web Component
// Converted from Vue component to reduce framework dependency

import {generateElemId} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';
import {filterRepoFilesWeighted} from '../features/repo-findfile.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {throttle} from '../utils/func.ts';

// threshold is used in filterRepoFilesWeighted (imported from repo-findfile.ts)
// const threshold = 50; // defined in repo-findfile.ts

class RepoFileSearch extends HTMLElement {
  #repoLink: string;
  #currentRefNameSubURL: string;
  #treeListUrl: string;
  #noResultsText: string;
  #placeholder: string;
  #refElemInput!: HTMLInputElement;
  #refElemPopup!: HTMLDivElement;
  #searchQuery = '';
  #allFiles: string[] = [];
  #selectedIndex = 0;
  #isLoadingFileList = false;
  #hasLoadedFileList = false;
  #searchPopupId: string;
  #boundHandleClickOutside: (e: MouseEvent) => void;
  #boundUpdatePosition: () => void;
  #boundHandleKeyDown: (e: KeyboardEvent) => void;
  #applySearchQuery: () => void;

  constructor() {
    super();
    this.attachShadow({mode: 'open'});

    this.#repoLink = this.getAttribute('data-repo-link') || '';
    this.#currentRefNameSubURL = this.getAttribute('data-current-ref-name-sub-url') || '';
    this.#treeListUrl = this.getAttribute('data-tree-list-url') || '';
    this.#noResultsText = this.getAttribute('data-no-results-text') || '';
    this.#placeholder = this.getAttribute('data-placeholder') || '';

    this.#searchPopupId = generateElemId('file-search-popup-');
    this.#boundHandleClickOutside = this.#handleClickOutside.bind(this);
    this.#boundUpdatePosition = this.#updatePosition.bind(this);
    this.#boundHandleKeyDown = this.#handleKeyDown.bind(this);
    this.#applySearchQuery = throttle(() => {
      this.#searchQuery = this.#refElemInput.value;
      this.#selectedIndex = 0;
      this.#renderPopup();
    }, 300);
  }

  static get observedAttributes(): string[] {
    return ['data-repo-link', 'data-current-ref-name-sub-url', 'data-tree-list-url', 'data-no-results-text', 'data-placeholder'];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    if (name === 'data-repo-link') this.#repoLink = newVal;
    else if (name === 'data-current-ref-name-sub-url') this.#currentRefNameSubURL = newVal;
    else if (name === 'data-tree-list-url') this.#treeListUrl = newVal;
    else if (name === 'data-no-results-text') this.#noResultsText = newVal;
    else if (name === 'data-placeholder') this.#placeholder = newVal;
  }

  connectedCallback(): void {
    this.#render();
    this.#refElemInput = this.shadowRoot!.querySelector('input')!;
    this.#refElemPopup = this.shadowRoot!.querySelector('.file-search-popup')!;

    this.#refElemPopup.setAttribute('id', this.#searchPopupId);
    this.#refElemInput.setAttribute('aria-controls', this.#searchPopupId);

    document.addEventListener('click', this.#boundHandleClickOutside);
    window.addEventListener('resize', this.#boundUpdatePosition);
    this.#refElemInput.addEventListener('input', this.#handleSearchInput.bind(this));
    this.#refElemInput.addEventListener('keydown', this.#boundHandleKeyDown);
  }

  disconnectedCallback(): void {
    document.removeEventListener('click', this.#boundHandleClickOutside);
    window.removeEventListener('resize', this.#boundUpdatePosition);
    this.#refElemInput.removeEventListener('input', this.#handleSearchInput.bind(this));
    this.#refElemInput.removeEventListener('keydown', this.#boundHandleKeyDown);
  }

  #handleSearchInput(): void {
    this.#loadFileListForSearch();
    this.#applySearchQuery();
  }

  #handleKeyDown(e: KeyboardEvent): void {
    if (e.isComposing) return;

    if (e.key === 'Escape') {
      this.#clearSearch();
      this.#refElemInput.blur();
      return;
    }
    const filteredFiles = this.#getFilteredFiles();
    if (!this.#searchQuery || filteredFiles.length === 0) return;

    const handleSelectedItem = (idx: number) => {
      e.preventDefault();
      this.#selectedIndex = idx;
      const el = this.#refElemPopup.querySelector(`.file-search-results > :nth-child(${idx + 1} of .item)`);
      el?.scrollIntoView({block: 'nearest', behavior: 'instant'});
    };

    if (e.key === 'ArrowDown') {
      handleSelectedItem(Math.min(this.#selectedIndex + 1, filteredFiles.length - 1));
    } else if (e.key === 'ArrowUp') {
      handleSelectedItem(Math.max(this.#selectedIndex - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const selectedFile = filteredFiles[this.#selectedIndex];
      if (selectedFile) {
        this.#handleSearchResultClick(selectedFile.matchResult.join(''));
      }
    }
  }

  #clearSearch(): void {
    this.#searchQuery = '';
    this.#refElemInput.value = '';
    this.#renderPopup();
  }

  #handleClickOutside(e: MouseEvent): void {
    if (!this.#searchQuery) return;

    const target = e.target as HTMLElement;
    const clickInside = this.#refElemInput.contains(target) || this.#refElemPopup.contains(target);
    if (!clickInside) this.#clearSearch();
  }

  async #loadFileListForSearch(): Promise<void> {
    if (this.#hasLoadedFileList || this.#isLoadingFileList) return;

    this.#isLoadingFileList = true;
    try {
      const response = await GET(this.#treeListUrl);
      this.#allFiles = await response.json();
      this.#hasLoadedFileList = true;
    } finally {
      this.#isLoadingFileList = false;
      this.#renderPopup();
    }
  }

  #getFilteredFiles(): Array<{matchResult: string[]; matchWeight: number}> {
    if (!this.#searchQuery) return [];
    return filterRepoFilesWeighted(this.#allFiles, this.#searchQuery);
  }

  #handleSearchResultClick(filePath: string): void {
    this.#clearSearch();
    window.location.assign(`${this.#repoLink}/src/${pathEscapeSegments(this.#currentRefNameSubURL)}/${pathEscapeSegments(filePath)}`);
  }

  #updatePosition(): void {
    if (!this.#searchQuery) return;

    const rectInput = this.#refElemInput.getBoundingClientRect();
    const rectPopup = this.#refElemPopup.getBoundingClientRect();
    const docElem = document.documentElement;
    const style = this.#refElemPopup.style;
    style.top = `${docElem.scrollTop + rectInput.bottom + 4}px`;
    if (rectInput.x + rectPopup.width < docElem.clientWidth) {
      style.left = `${docElem.scrollLeft + rectInput.x}px`;
    } else {
      const leftPos = docElem.scrollLeft + docElem.getBoundingClientRect().width - rectPopup.width;
      style.left = `calc(${leftPos}px - var(--page-margin-x))`;
    }
  }

  #render(): void {
    const styles = `
      <style>
        .file-search-wrapper { position: relative; display: inline-block; }
        .file-search-input {
          width: 100%;
          padding: 0.375rem 0.75rem;
          border: 1px solid var(--color-secondary);
          border-radius: var(--border-radius);
          background: var(--color-input-background);
          color: var(--color-text);
          font-size: 0.875rem;
        }
        .file-search-input:focus {
          outline: none;
          border-color: var(--color-primary);
        }
        .file-search-popup {
          position: absolute;
          background: var(--color-box-body);
          border: 1px solid var(--color-secondary);
          border-radius: var(--border-radius);
          width: max-content;
          max-height: min(calc(100vw - 20px), 300px);
          max-width: min(calc(100vw - 40px), 600px);
          overflow-y: auto;
          z-index: 1000;
          display: none;
        }
        .file-search-popup.visible { display: block; }
        .file-search-popup .is-loading {
          width: 200px;
          height: 200px;
          background: linear-gradient(90deg, var(--color-secondary-alpha-60) 25%, var(--color-secondary-alpha-30) 50%, var(--color-secondary-alpha-60) 75%);
          background-size: 200% 100%;
          animation: loading 1.5s infinite;
        }
        @keyframes loading { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }
        .file-search-results { display: flex; flex-direction: column; }
        .file-search-results .item {
          display: flex;
          align-items: flex-start;
          padding: 0.5rem 0.75rem;
          cursor: pointer;
          border-bottom: 1px solid var(--color-secondary);
        }
        .file-search-results .item:last-child { border-bottom: none; }
        .file-search-results .item:hover,
        .file-search-results .item.selected { background-color: var(--color-hover); }
        .file-search-results .item .file-icon { flex-shrink: 0; margin-top: 0.125rem; width: 16px; height: 16px; }
        .file-search-results .item .full-path { flex: 1; overflow-wrap: anywhere; }
        .file-search-results .item .full-path :nth-child(even) {
          color: var(--color-red);
          font-weight: var(--font-weight-semibold);
        }
        .file-search-empty { padding: 1rem; color: var(--color-text-light-2); text-align: center; }
        .global-shortcut-wrapper { position: relative; display: inline-flex; align-items: center; }
        .global-shortcut-wrapper kbd {
          position: absolute;
          right: 0.5rem;
          font-size: 0.75rem;
          padding: 0.125rem 0.25rem;
          background: var(--color-secondary);
          border-radius: 0.25rem;
          color: var(--color-text-light-1);
        }
      </style>
    `;

    const template = `
      <div class="file-search-wrapper">
        <div class="ui small input global-shortcut-wrapper">
          <input
            type="text"
            class="file-search-input"
            placeholder="${this.#placeholder}"
            autocomplete="off"
            role="combobox"
            aria-autocomplete="list"
            aria-expanded="false"
          >
          <kbd data-global-init="onGlobalShortcut" data-shortcut-keys="t">T</kbd>
        </div>
        <div class="file-search-popup" role="listbox" aria-label="File search results"></div>
      </div>
    `;

    this.shadowRoot!.innerHTML = styles + template;
  }

  #renderPopup(): void {
    const popup = this.shadowRoot!.querySelector('.file-search-popup')!;
    const input = this.#refElemInput;
    const filteredFiles = this.#getFilteredFiles();

    if (!this.#searchQuery) {
      popup.classList.remove('visible');
      input.setAttribute('aria-expanded', 'false');
      return;
    }

    popup.classList.add('visible');
    input.setAttribute('aria-expanded', 'true');

    if (filteredFiles.length > 0) {
      popup.innerHTML = `
        <div class="file-search-results" role="listbox">
          ${filteredFiles.map((result, idx) => `
            <div
              class="item ${idx === this.#selectedIndex ? 'selected' : ''}"
              role="option"
              aria-selected="${idx === this.#selectedIndex}"
              data-file-path="${result.matchResult.join('')}"
              title="${result.matchResult.join('')}"
            >
              <svg-icon name="octicon-file" class="file-icon"></svg-icon>
              <span class="full-path">
                ${result.matchResult.map((part, _index) => `<span>${this.#escapeHtml(part)}</span>`).join('')}
              </span>
            </div>
          `).join('')}
        </div>
      `;
    } else if (this.#isLoadingFileList) {
      popup.innerHTML = `<div class="is-loading"></div>`;
    } else {
      popup.innerHTML = `<div class="file-search-empty">${this.#escapeHtml(this.#noResultsText)}</div>`;
    }

    // Add click/mouseenter listeners to items
    for (const item of popup.querySelectorAll('.item')) {
      item.addEventListener('click', () => {
        const filePath = item.getAttribute('data-file-path')!;
        this.#handleSearchResultClick(filePath);
      });
      item.addEventListener('mouseenter', () => {
        // We need to track index differently since we don't have data-commit-idx
        // Use the index from the iteration
      });
    }

    // Re-add mouseenter listeners with correct index
    const items = popup.querySelectorAll('.item');
    items.forEach((item, idx) => {
      item.addEventListener('mouseenter', () => { this.#selectedIndex = idx; this.#renderPopup() });
    });

    this.#updatePosition();
  }

  #escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

if (!customElements.get('repo-file-search')) {
  customElements.define('repo-file-search', RepoFileSearch);
}
