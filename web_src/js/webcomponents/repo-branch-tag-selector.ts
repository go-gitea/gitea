// RepoBranchTagSelector Web Component
// Converted from Vue component to reduce framework dependency

import {showErrorToast} from '../modules/toast.ts';
import {GET} from '../modules/fetch.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {queryElemChildren} from '../utils/dom.ts';
import {trString} from '../modules/i18n.ts';

type GitRefType = 'branch' | 'tag' | 'commit';

type ListItem = {
  selected: boolean;
  refShortName: string;
  refType: GitRefType;
  rssFeedLink: string;
};

type SelectedTab = 'branches' | 'tags';
type TabLoadingStates = Record<SelectedTab, '' | 'loading' | 'done'>;

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

class RepoBranchTagSelector extends HTMLElement {
  #elDropdown!: HTMLElement;
  #elCreateNewRefForm!: HTMLFormElement;
  #elScrollContainer!: HTMLDivElement;
  #elSearchField!: HTMLInputElement;
  #elBranchTab!: HTMLButtonElement | null;
  #elTagTab!: HTMLButtonElement | null;

  #showTabBranches: boolean;
  #showTabTags: boolean;
  #allowCreateNewRef: boolean;
  #showViewAllRefsEntry: boolean;
  #enableFeed: boolean;
  #refFormActionTemplate: string;
  #refLinkTemplate: string;
  #dropdownFixedText: string;
  #currentRepoDefaultBranch: string;
  #currentRepoLink: string;
  #currentTreePath: string;
  #currentRefType: GitRefType;
  #currentRefShortName: string;

  #textBranches: string;
  #textTags: string;
  #textFilterBranch: string;
  #textFilterTag: string;
  #textDefaultBranchLabel: string;
  #textCreateTag: string;
  #textCreateBranch: string;
  #textCreateRefFrom: string;
  #textNoResults: string;
  #textViewAllBranches: string;
  #textViewAllTags: string;

  #allItems: ListItem[] = [];
  #selectedTab: SelectedTab;
  #searchTerm = '';
  #menuVisible = false;
  #activeItemIndex = 0;
  #tabLoadingStates: TabLoadingStates = {branches: '', tags: ''};
  #boundOnBodyClick: (e: MouseEvent) => void;
  #boundOnKeyDown: (e: KeyboardEvent) => void;

  constructor() {
    super();
    this.attachShadow({mode: 'open'});

    // Read all data attributes
    this.#showTabBranches = this.getAttribute('data-show-tab-branches') === 'true';
    this.#showTabTags = this.getAttribute('data-show-tab-tags') === 'true';
    this.#allowCreateNewRef = this.getAttribute('data-allow-create-new-ref') === 'true';
    this.#showViewAllRefsEntry = this.getAttribute('data-show-view-all-refs-entry') === 'true';
    this.#enableFeed = this.getAttribute('data-enable-feed') === 'true';
    this.#refFormActionTemplate = this.getAttribute('data-ref-form-action-template') || '';
    this.#refLinkTemplate = this.getAttribute('data-ref-link-template') || '';
    this.#dropdownFixedText = this.getAttribute('data-dropdown-fixed-text') || '';
    this.#currentRepoDefaultBranch = this.getAttribute('data-current-repo-default-branch') || '';
    this.#currentRepoLink = this.getAttribute('data-current-repo-link') || '';
    this.#currentTreePath = this.getAttribute('data-current-tree-path') || '';
    this.#currentRefType = (this.getAttribute('data-current-ref-type') as GitRefType) || 'branch';
    this.#currentRefShortName = this.getAttribute('data-current-ref-short-name') || '';

    this.#textBranches = this.getAttribute('data-text-branches') || '';
    this.#textTags = this.getAttribute('data-text-tags') || '';
    this.#textFilterBranch = this.getAttribute('data-text-filter-branch') || '';
    this.#textFilterTag = this.getAttribute('data-text-filter-tag') || '';
    this.#textDefaultBranchLabel = this.getAttribute('data-text-default-branch-label') || '';
    this.#textCreateTag = this.getAttribute('data-text-create-tag') || '';
    this.#textCreateBranch = this.getAttribute('data-text-create-branch') || '';
    this.#textCreateRefFrom = this.getAttribute('data-text-create-ref-from') || '';
    this.#textNoResults = this.getAttribute('data-text-no-results') || '';
    this.#textViewAllBranches = this.getAttribute('data-text-view-all-branches') || '';
    this.#textViewAllTags = this.getAttribute('data-text-view-all-tags') || '';

    this.#selectedTab = this.#showTabBranches ? 'branches' : 'tags';

    this.#boundOnBodyClick = this.#onBodyClick.bind(this);
    this.#boundOnKeyDown = this.#onKeyDown.bind(this);
  }

  static get observedAttributes(): string[] {
    return [
      'data-show-tab-branches', 'data-show-tab-tags', 'data-allow-create-new-ref',
      'data-show-view-all-refs-entry', 'data-enable-feed', 'data-ref-form-action-template',
      'data-ref-link-template', 'data-dropdown-fixed-text', 'data-current-repo-default-branch',
      'data-current-repo-link', 'data-current-tree-path', 'data-current-ref-type',
      'data-current-ref-short-name', 'data-text-branches', 'data-text-tags',
      'data-text-filter-branch', 'data-text-filter-tag', 'data-text-default-branch-label',
      'data-text-create-tag', 'data-text-create-branch', 'data-text-create-ref-from',
      'data-text-no-results', 'data-text-view-all-branches', 'data-text-view-all-tags',
    ];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    switch (name) {
      case 'data-show-tab-branches': this.#showTabBranches = newVal === 'true'; break;
      case 'data-show-tab-tags': this.#showTabTags = newVal === 'true'; break;
      case 'data-allow-create-new-ref': this.#allowCreateNewRef = newVal === 'true'; break;
      case 'data-show-view-all-refs-entry': this.#showViewAllRefsEntry = newVal === 'true'; break;
      case 'data-enable-feed': this.#enableFeed = newVal === 'true'; break;
      case 'data-ref-form-action-template': this.#refFormActionTemplate = newVal; break;
      case 'data-ref-link-template': this.#refLinkTemplate = newVal; break;
      case 'data-dropdown-fixed-text': this.#dropdownFixedText = newVal; break;
      case 'data-current-repo-default-branch': this.#currentRepoDefaultBranch = newVal; break;
      case 'data-current-repo-link': this.#currentRepoLink = newVal; break;
      case 'data-current-tree-path': this.#currentTreePath = newVal; break;
      case 'data-current-ref-type': this.#currentRefType = newVal as GitRefType; break;
      case 'data-current-ref-short-name': this.#currentRefShortName = newVal; break;
      case 'data-text-branches': this.#textBranches = newVal; break;
      case 'data-text-tags': this.#textTags = newVal; break;
      case 'data-text-filter-branch': this.#textFilterBranch = newVal; break;
      case 'data-text-filter-tag': this.#textFilterTag = newVal; break;
      case 'data-text-default-branch-label': this.#textDefaultBranchLabel = newVal; break;
      case 'data-text-create-tag': this.#textCreateTag = newVal; break;
      case 'data-text-create-branch': this.#textCreateBranch = newVal; break;
      case 'data-text-create-ref-from': this.#textCreateRefFrom = newVal; break;
      case 'data-text-no-results': this.#textNoResults = newVal; break;
      case 'data-text-view-all-branches': this.#textViewAllBranches = newVal; break;
      case 'data-text-view-all-tags': this.#textViewAllTags = newVal; break;
    }
    this.#render();
  }

  connectedCallback(): void {
    this.#render();
    this.#elDropdown = this.shadowRoot!.querySelector('.branch-selector-dropdown')!;
    this.#elCreateNewRefForm = this.shadowRoot!.querySelector('#create-new-ref-form')!;
    this.#elScrollContainer = this.shadowRoot!.querySelector('.scrolling.menu')!;
    this.#elSearchField = this.shadowRoot!.querySelector('input[name="search"]')!;
    this.#elBranchTab = this.shadowRoot!.querySelector('[data-tab="branches"]');
    this.#elTagTab = this.shadowRoot!.querySelector('[data-tab="tags"]');

    document.body.addEventListener('click', this.#boundOnBodyClick);
    this.#elDropdown.addEventListener('keydown', this.#boundOnKeyDown);

    // Handle tab clicks
    this.#elBranchTab?.addEventListener('click', () => this.#handleTabSwitch('branches'));
    this.#elTagTab?.addEventListener('click', () => this.#handleTabSwitch('tags'));

    // Handle search input
    this.#elSearchField.addEventListener('input', () => {
      this.#searchTerm = this.#elSearchField.value;
      this.#activeItemIndex = 0;
      this.#renderMenu();
    });

    // Handle dropdown button click
    const dropdownBtn = this.shadowRoot!.querySelector('.branch-dropdown-button');
    dropdownBtn?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.#menuVisible = !this.#menuVisible;
      this.#renderMenu();
      if (this.#menuVisible) {
        this.#focusSearchField();
        this.#loadTabItems();
      }
    });

    // Handle form submission for create new ref
    this.#elCreateNewRefForm?.addEventListener('submit', () => {
      // Form submits naturally
    });

    // Handle create new ref click
    const createNewRefItem = this.shadowRoot!.querySelector('.create-new-ref-item');
    createNewRefItem?.addEventListener('click', () => this.#createNewRef());

    // Handle item clicks
    this.#elScrollContainer.addEventListener('click', (e) => {
      const item = (e.target as HTMLElement).closest('.item[data-ref-short-name]');
      if (item) {
        const refShortName = item.getAttribute('data-ref-short-name')!;
        const refType = item.getAttribute('data-ref-type') as GitRefType;
        this.#selectItem({refShortName, refType, selected: false, rssFeedLink: ''});
      }
    });

    // Handle view all links
    const viewAllBranches = this.shadowRoot!.querySelector('[data-view-all="branches"]');
    const viewAllTags = this.shadowRoot!.querySelector('[data-view-all="tags"]');
    viewAllBranches?.addEventListener('click', (e) => {
      e.preventDefault();
      window.location.assign(`${this.#currentRepoLink}/branches`);
    });
    viewAllTags?.addEventListener('click', (e) => {
      e.preventDefault();
      window.location.assign(`${this.#currentRepoLink}/tags`);
    });

    // If used in form, initialize with current selection
    if (this.#refFormActionTemplate) {
      this.#selectItem({
        refType: this.#currentRefType,
        refShortName: this.#currentRefShortName,
        selected: true,
        rssFeedLink: '',
      });
    }
  }

  disconnectedCallback(): void {
    document.body.removeEventListener('click', this.#boundOnBodyClick);
    this.#elDropdown.removeEventListener('keydown', this.#boundOnKeyDown);
  }

  #onBodyClick(e: MouseEvent): void {
    if (this.#elDropdown.contains(e.target as Node)) return;
    if (this.#menuVisible) {
      this.#menuVisible = false;
      this.#renderMenu();
    }
  }

  #onKeyDown(e: KeyboardEvent): void {
    if (e.isComposing) return;
    if (!this.#menuVisible) return;

    if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      e.preventDefault();
      const filteredItems = this.#getFilteredItems();

      if (this.#activeItemIndex === -1) {
        this.#activeItemIndex = this.#getSelectedIndexInFiltered();
      }
      const nextIndex = e.key === 'ArrowDown' ? this.#activeItemIndex + 1 : this.#activeItemIndex - 1;
      if (nextIndex < 0) return;
      if (nextIndex + (this.#showCreateNewRef() ? 0 : 1) > filteredItems.length) return;
      this.#activeItemIndex = nextIndex;
      this.#getActiveItem()?.scrollIntoView({block: 'nearest'});
    } else if (e.key === 'Enter') {
      e.preventDefault();
      this.#getActiveItem()?.click();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      this.#menuVisible = false;
      this.#renderMenu();
    }
  }

  #focusSearchField(): void {
    queueMicrotask(() => {
      this.#elSearchField.focus();
    });
  }

  #getFilteredItems(): ListItem[] {
    const searchTermLower = this.#searchTerm.toLowerCase();
    return this.#allItems.filter((item) => {
      const typeMatched = (this.#selectedTab === 'branches' && item.refType === 'branch') ||
                          (this.#selectedTab === 'tags' && item.refType === 'tag');
      if (!typeMatched) return false;
      if (!this.#searchTerm) return true;
      return item.refShortName.toLowerCase().includes(searchTermLower);
    });
  }

  #showCreateNewRef(): boolean {
    if (!this.#allowCreateNewRef || !this.#searchTerm) return false;
    return this.#allItems.every((item) => item.refShortName !== this.#searchTerm);
  }

  #showNoResults(): boolean {
    if (this.#tabLoadingStates[this.#selectedTab] !== 'done') return false;
    return this.#getFilteredItems().length === 0 && !this.#showCreateNewRef();
  }

  #getSelectedIndexInFiltered(): number {
    return this.#getFilteredItems().findIndex((item) => item.selected);
  }

  #getActiveItem(): HTMLElement | null {
    return queryElemChildren<HTMLDivElement>(this.#elScrollContainer, '.item')[this.#activeItemIndex] ?? null;
  }

  #selectItem(item: ListItem): void {
    this.#menuVisible = false;
    this.#renderMenu();

    if (this.#refFormActionTemplate) {
      this.#currentRefType = item.refType;
      this.#currentRefShortName = item.refShortName;
      const form = this.#elDropdown.closest('form')!;
      form.action = this.#refFormActionTemplate
        .replace('{RepoLink}', this.#currentRepoLink)
        .replace('{RefType}', pathEscapeSegments(item.refType))
        .replace('{RefShortName}', pathEscapeSegments(item.refShortName));
    } else {
      window.location.assign(this.#refLinkTemplate
        .replace('{RepoLink}', this.#currentRepoLink)
        .replace('{RefType}', pathEscapeSegments(item.refType))
        .replace('{RefShortName}', pathEscapeSegments(item.refShortName))
        .replace('{TreePath}', pathEscapeSegments(this.#currentTreePath)));
    }
  }

  #createNewRef(): void {
    this.#elCreateNewRefForm?.submit();
  }

  #handleTabSwitch(tab: SelectedTab): void {
    this.#selectedTab = tab;
    this.#focusSearchField();
    this.#loadTabItems();
    this.#renderTabs();
  }

  async #loadTabItems(): Promise<void> {
    const tab = this.#selectedTab;
    if (this.#tabLoadingStates[tab] === 'loading' || this.#tabLoadingStates[tab] === 'done') return;

    const refType = tab === 'branches' ? 'branch' : 'tag';
    this.#tabLoadingStates = {...this.#tabLoadingStates, [tab]: 'loading'};
    this.#renderMenu();

    try {
      const resp = await GET(`${this.#currentRepoLink}/${tab}/list`);
      const {results} = await resp.json() as {results: string[]};
      const newItems = results.map((refShortName): ListItem => ({
        refType,
        refShortName,
        selected: refType === this.#currentRefType && refShortName === this.#currentRefShortName,
        rssFeedLink: `${this.#currentRepoLink}/rss/${refType}/${pathEscapeSegments(refShortName)}`,
      }));
      this.#allItems = [...this.#allItems, ...newItems];
      this.#tabLoadingStates = {...this.#tabLoadingStates, [tab]: 'done'};
    } catch (e) {
      this.#tabLoadingStates = {...this.#tabLoadingStates, [tab]: ''};
      showErrorToast(`Network error when fetching items for ${tab}, error: ${e}`);
      console.error(e);
    }
    this.#renderMenu();
  }

  #render(): void {
    const styles = `
      <style>
        .branch-selector-dropdown { position: relative; display: inline-block; }
        .branch-dropdown-button { display: flex; align-items: center; gap: 0.5rem; padding: 0.375rem 0.75rem; }
        .flex-text-block { display: flex; align-items: center; }
        .gt-ellipsis { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        .dropdown-icon { flex-shrink: 0; }
        .menu {
          position: absolute;
          top: 100%;
          left: 0;
          margin-top: 0.25em;
          background: var(--color-box-body);
          border: 1px solid var(--color-secondary);
          border-radius: var(--border-radius);
          box-shadow: var(--shadow-lg);
          z-index: 100;
          min-width: 200px;
          max-height: 400px;
          overflow: hidden;
          display: none;
        }
        .menu.visible { display: block; }
        .ui.icon.search.input { position: relative; padding: 0.5rem; }
        .ui.icon.search.input .icon { position: absolute; left: 0.75rem; top: 50%; transform: translateY(-50%); color: var(--color-text-light-2); }
        .ui.icon.search.input input { width: 100%; padding: 0.375rem 0.75rem 0.375rem 2.5rem; border: 1px solid var(--color-secondary); border-radius: var(--border-radius); background: var(--color-input-background); color: var(--color-text); font-size: 0.875rem; }
        .branch-tag-tab { display: flex; gap: 0.25rem; padding: 0.5rem; border-bottom: 1px solid var(--color-secondary); }
        .branch-tag-item { padding: 0.375rem 0.75rem; border: none; background: transparent; color: var(--color-text-light-1); cursor: pointer; display: flex; align-items: center; gap: 0.25rem; }
        .branch-tag-item.active { color: var(--color-primary); font-weight: var(--font-weight-semibold); }
        .branch-tag-divider { height: 1px; background: var(--color-secondary); margin: 0.25rem 0; }
        .scrolling.menu { max-height: 300px; overflow-y: auto; padding: 0.25rem 0; }
        .loading-indicator { height: 100px; background: linear-gradient(90deg, var(--color-secondary-alpha-60) 25%, var(--color-secondary-alpha-30) 50%, var(--color-secondary-alpha-60) 75%); background-size: 200% 100%; animation: loading 1.5s infinite; }
        @keyframes loading { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }
        .menu .item { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; padding: 0.375rem 0.75rem; cursor: pointer; }
        .menu .item:hover, .menu .item.active { background: var(--color-hover); }
        .menu .item.selected { font-weight: var(--font-weight-semibold); }
        .menu .item .ui.label { font-size: 0.75rem; padding: 0.125rem 0.375rem; }
        .menu .item .rss-icon { color: var(--color-text-light-2); cursor: pointer; }
        .menu .item .rss-icon:hover { color: var(--color-primary); }
        .menu .message { padding: 0.75rem; color: var(--color-text-light-2); text-align: center; }
        .menu .divider { height: 1px; background: var(--color-secondary); margin: 0; }
        .menu .item[data-view-all] { padding: 0.5rem 0.75rem; color: var(--color-text-light-1); }
        .menu .item[data-view-all]:hover { color: var(--color-primary); background: var(--color-hover); }
        .create-new-ref-item { display: flex; flex-direction: column; gap: 0.25rem; padding: 0.5rem 0.75rem; cursor: pointer; }
        .create-new-ref-item:hover { background: var(--color-hover); }
        .create-new-ref-item .tw-text-xs { font-size: 0.75rem; color: var(--color-text-light-2); }
        .ellipsis-text-items .gt-ellipsis { max-width: 200px; }
      </style>
    `;

    const template = `
      <div class="ui dropdown custom branch-selector-dropdown ellipsis-text-items">
        <div tabindex="0" class="ui compact button branch-dropdown-button">
          <span class="flex-text-block gt-ellipsis">
            ${this.#dropdownFixedText ? escapeHtml(this.#dropdownFixedText) : `
              ${this.#currentRefType === 'tag' ? '<svg-icon name="octicon-tag"></svg-icon>' : ''}
              ${this.#currentRefType === 'branch' ? '<svg-icon name="octicon-git-branch"></svg-icon>' : ''}
              ${this.#currentRefType === 'commit' ? '<svg-icon name="octicon-git-commit"></svg-icon>' : ''}
              <strong class="tw-inline-block gt-ellipsis">${escapeHtml(this.#currentRefShortName)}</strong>
            `}
          </span>
          <svg-icon name="octicon-triangle-down" size="14" class="dropdown-icon"></svg-icon>
        </div>
        <div class="menu transition ${this.#menuVisible ? 'visible' : ''}" role="menu">
          <div class="ui icon search input">
            <i class="icon"><svg-icon name="octicon-filter" size="16"></svg-icon></i>
            <input name="search" autocomplete="off" placeholder="${escapeHtml(this.#selectedTab === 'branches' ? this.#textFilterBranch : this.#textFilterTag)}">
          </div>
          ${this.#showTabBranches ? `
            <div class="branch-tag-tab">
              <button type="button" class="btn branch-tag-item ${this.#selectedTab === 'branches' ? 'active' : ''}" data-tab="branches">
                <svg-icon name="octicon-git-branch" size="16" class="tw-mr-1"></svg-icon>${escapeHtml(this.#textBranches)}
              </button>
              ${this.#showTabTags ? `
                <button type="button" class="btn branch-tag-item ${this.#selectedTab === 'tags' ? 'active' : ''}" data-tab="tags">
                  <svg-icon name="octicon-tag" size="16" class="tw-mr-1"></svg-icon>${escapeHtml(this.#textTags)}
                </button>
              ` : ''}
            </div>
          ` : ''}
          <div class="branch-tag-divider"></div>
          <div class="scrolling menu" role="listbox">
            <svg-icon name="octicon-rss" symbol-id="svg-symbol-octicon-rss"></svg-icon>
            ${this.#tabLoadingStates[this.#selectedTab] === 'loading' ? '<div class="loading-indicator is-loading"></div>' : ''}
            ${this.#getFilteredItems().map((item, index) => `
              <div class="item ${item.selected ? 'selected' : ''} ${this.#activeItemIndex === index ? 'active' : ''}" role="option" data-ref-short-name="${escapeHtml(item.refShortName)}" data-ref-type="${item.refType}" aria-selected="${item.selected}">
                ${escapeHtml(item.refShortName)}
                ${item.refType === 'branch' && item.refShortName === this.#currentRepoDefaultBranch ? `<div class="ui label">${escapeHtml(this.#textDefaultBranchLabel)}</div>` : ''}
                ${this.#enableFeed && this.#selectedTab === 'branches' ? `<a role="button" class="rss-icon" target="_blank" href="${item.rssFeedLink}" onclick="event.stopPropagation()"><svg width="14" height="14" class="svg octicon-rss"><use href="#svg-symbol-octicon-rss"></use></svg></a>` : ''}
              </div>
            `).join('')}
            ${this.#showCreateNewRef() ? `
              <div class="item create-new-ref-item" role="option" aria-selected="false">
                ${this.#selectedTab === 'tags' ? `
                  <div><svg-icon name="octicon-tag" class="tw-mr-1"></svg-icon><span>${escapeHtml(trString(this.#textCreateTag, this.#searchTerm))}</span></div>
                ` : `
                  <div><svg-icon name="octicon-git-branch" class="tw-mr-1"></svg-icon><span>${escapeHtml(trString(this.#textCreateBranch, this.#searchTerm))}</span></div>
                `}
                <div class="tw-text-xs">${escapeHtml(this.#textCreateRefFrom.replace('%s', this.#currentRefShortName))}</div>
                <form id="create-new-ref-form" method="post" action="${this.#currentRepoLink}/branches/_new/${this.#currentRefType}/${pathEscapeSegments(this.#currentRefShortName)}">
                  <input type="hidden" name="new_branch_name" value="${escapeHtml(this.#searchTerm)}">
                  <input type="hidden" name="create_tag" value="${this.#selectedTab === 'tags' ? 'true' : 'false'}">
                  <input type="hidden" name="current_path" value="${escapeHtml(this.#currentTreePath)}">
                </form>
              </div>
            ` : ''}
            ${this.#showNoResults() ? `<div class="message">${escapeHtml(this.#textNoResults)}</div>` : ''}
            ${this.#showViewAllRefsEntry ? `
              <div class="divider tw-m-0"></div>
              <a class="item" data-view-all="branches" ${this.#selectedTab !== 'branches' ? 'style="display:none"' : ''} href="${this.#currentRepoLink}/branches">${escapeHtml(this.#textViewAllBranches)}</a>
              <a class="item" data-view-all="tags" ${this.#selectedTab !== 'tags' ? 'style="display:none"' : ''} href="${this.#currentRepoLink}/tags">${escapeHtml(this.#textViewAllTags)}</a>
            ` : ''}
          </div>
        </div>
      </div>
    `;

    this.shadowRoot!.innerHTML = styles + template;
  }

  #renderMenu(): void {
    const menu = this.shadowRoot!.querySelector('.menu');
    const dropdownBtn = this.shadowRoot!.querySelector('.branch-dropdown-button');
    if (!menu || !dropdownBtn) return;

    menu.classList.toggle('visible', this.#menuVisible);
    dropdownBtn.setAttribute('aria-expanded', this.#menuVisible ? 'true' : 'false');
    menu.setAttribute('aria-expanded', this.#menuVisible ? 'true' : 'false');

    // Update tab button states
    this.#elBranchTab?.classList.toggle('active', this.#selectedTab === 'branches');
    this.#elTagTab?.classList.toggle('active', this.#selectedTab === 'tags');

    // Update search placeholder
    this.#elSearchField.placeholder = this.#selectedTab === 'branches' ? this.#textFilterBranch : this.#textFilterTag;

    if (this.#menuVisible) {
      this.#focusSearchField();
      this.#loadTabItems();
    }
  }

  #renderTabs(): void {
    this.#elBranchTab?.classList.toggle('active', this.#selectedTab === 'branches');
    this.#elTagTab?.classList.toggle('active', this.#selectedTab === 'tags');
    this.#elSearchField.placeholder = this.#selectedTab === 'branches' ? this.#textFilterBranch : this.#textFilterTag;
  }
}

if (!customElements.get('repo-branch-tag-selector')) {
  customElements.define('repo-branch-tag-selector', RepoBranchTagSelector);
}
