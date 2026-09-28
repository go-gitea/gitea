// DiffCommitSelector Web Component
// Converted from Vue component to reduce framework dependency

import {generateElemId} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';

type Commit = {
  id: string;
  hovered: boolean;
  selected: boolean;
  summary: string;
  committer_or_author_name: string;
  time: string;
  short_sha: string;
};

type CommitListResult = {
  commits: Commit[];
  last_review_commit_sha: string;
  locale: Record<string, string>;
};

class DiffCommitSelector extends HTMLElement {
  #elRoot: HTMLElement;
  #elExpandBtn: HTMLButtonElement;
  #elShowAllChanges: HTMLDivElement;
  #queryParams: string;
  #issueLink: string;
  #mergeBase: string;
  #uniqueIdMenu: string;
  #uniqueIdShowAll: string;
  #menuVisible = false;
  #isLoading = false;
  #locale: Record<string, string> = {filter_changes_by_commit: ''};
  #commits: Commit[] = [];
  #hoverActivated = false;
  #lastReviewCommitSha: string | null = null;
  #boundOnBodyClick: (e: MouseEvent) => void;
  #boundOnKeyDown: (e: KeyboardEvent) => void;
  #boundOnKeyUp: (e: KeyboardEvent) => void;

  constructor() {
    super();
    this.attachShadow({mode: 'open'});

    this.#elRoot = this.shadowRoot!.querySelector('.diff-commit-selector')!;
    this.#elExpandBtn = this.shadowRoot!.querySelector('.expand-btn')!;
    this.#elShowAllChanges = this.shadowRoot!.querySelector('.show-all-changes')!;

    this.#queryParams = this.getAttribute('data-queryparams') || '';
    this.#issueLink = this.getAttribute('data-issuelink') || '';
    this.#mergeBase = this.getAttribute('data-merge-base') || '';
    this.#uniqueIdMenu = generateElemId('diff-commit-selector-menu-');
    this.#uniqueIdShowAll = generateElemId('diff-commit-selector-show-all-');

    this.#locale = {filter_changes_by_commit: this.getAttribute('data-text-filter-changes-by-commit') || ''};

    this.#boundOnBodyClick = this.#onBodyClick.bind(this);
    this.#boundOnKeyDown = this.#onKeyDown.bind(this);
    this.#boundOnKeyUp = this.#onKeyUp.bind(this);
  }

  static get observedAttributes(): string[] {
    return ['data-queryparams', 'data-issuelink', 'data-merge-base', 'data-text-filter-changes-by-commit'];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    if (name === 'data-queryparams') this.#queryParams = newVal;
    else if (name === 'data-issuelink') this.#issueLink = newVal;
    else if (name === 'data-merge-base') this.#mergeBase = newVal;
    else if (name === 'data-text-filter-changes-by-commit') this.#locale.filter_changes_by_commit = newVal;
  }

  connectedCallback(): void {
    this.#render();
    this.#elRoot = this.shadowRoot!.querySelector('.diff-commit-selector')!;
    this.#elExpandBtn = this.shadowRoot!.querySelector('.expand-btn')!;
    this.#elShowAllChanges = this.shadowRoot!.querySelector('.show-all-changes')!;

    document.body.addEventListener('click', this.#boundOnBodyClick);
    this.#elRoot.addEventListener('keydown', this.#boundOnKeyDown);
    this.#elRoot.addEventListener('keyup', this.#boundOnKeyUp);
  }

  disconnectedCallback(): void {
    document.body.removeEventListener('click', this.#boundOnBodyClick);
    this.#elRoot.removeEventListener('keydown', this.#boundOnKeyDown);
    this.#elRoot.removeEventListener('keyup', this.#boundOnKeyUp);
  }

  #onBodyClick(event: MouseEvent): void {
    if (this.#elRoot.contains(event.target as Node)) return;
    if (this.#menuVisible) this.#toggleMenu();
  }

  #onKeyDown(event: KeyboardEvent): void {
    if (!this.#menuVisible) return;
    const item = document.activeElement as HTMLElement;
    if (!this.#elRoot.contains(item)) return;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.#focusElem(item.nextElementSibling as HTMLElement, item);
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.#focusElem(item.previousElementSibling as HTMLElement, item);
        break;
      case 'Escape':
        event.preventDefault();
        if (item) item.tabIndex = -1;
        this.#toggleMenu();
        break;
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      const activeItem = document.activeElement;
      const commitIdx = activeItem?.matches('.item') ? activeItem.getAttribute('data-commit-idx') : null;
      if (commitIdx) this.#highlight(this.#commits[Number(commitIdx)]);
    }
  }

  #onKeyUp(event: KeyboardEvent): void {
    if (!this.#menuVisible) return;
    const item = document.activeElement;
    if (!this.#elRoot.contains(item)) return;
    if (event.key === 'Shift' && this.#hoverActivated) {
      this.#hoverActivated = false;
      for (const commit of this.#commits) {
        commit.hovered = false;
        commit.selected = false;
      }
      this.#renderMenu();
    }
  }

  #focusElem(elem: HTMLElement | null, prevElem: HTMLElement | null): void {
    if (elem) {
      elem.tabIndex = 0;
      if (prevElem) prevElem.tabIndex = -1;
      elem.focus();
    }
  }

  #toggleMenu(): void {
    this.#menuVisible = !this.#menuVisible;
    if (!this.#commits.length && this.#menuVisible && !this.#isLoading) {
      this.#isLoading = true;
      this.#fetchCommits().finally(() => { this.#isLoading = false });
    }
    this.#renderMenu();
    queueMicrotask(() => {
      if (this.#menuVisible) {
        this.#focusElem(this.#elShowAllChanges, this.#elExpandBtn);
      } else {
        this.#focusElem(this.#elExpandBtn, this.#elShowAllChanges);
      }
    });
  }

  async #fetchCommits(): Promise<void> {
    try {
      const resp = await GET(`${this.#issueLink}/commits/list`);
      const results = await resp.json() as CommitListResult;
      for (const commit of results.commits) commit.hovered = false;
      this.#commits.push(...results.commits);
      this.#commits.reverse();
      this.#lastReviewCommitSha = results.last_review_commit_sha || null;
      if (this.#lastReviewCommitSha && this.#commits.every((x) => x.id !== this.#lastReviewCommitSha)) {
        this.#lastReviewCommitSha = null;
      }
      this.#locale = {...this.#locale, ...results.locale};
    } catch (e) {
      console.error('Failed to fetch commits:', e);
    }
    this.#renderMenu();
  }

  get #commitsSinceLastReview(): number {
    if (this.#lastReviewCommitSha) {
      return this.#commits.length - this.#commits.findIndex((x) => x.id === this.#lastReviewCommitSha) - 1;
    }
    return 0;
  }

  #showAllChanges(): void {
    window.location.assign(`${this.#issueLink}/files${this.#queryParams}`);
  }

  #changesSinceLastReviewClick(): void {
    if (!this.#lastReviewCommitSha) return;
    window.location.assign(`${this.#issueLink}/files/${this.#lastReviewCommitSha}..${this.#commits.at(-1)!.id}${this.#queryParams}`);
  }

  #commitClicked(commitId: string, newWindow = false): void {
    const url = `${this.#issueLink}/commits/${commitId}${this.#queryParams}`;
    if (newWindow) window.open(url);
    else window.location.assign(url);
  }

  #commitClickedShift(commit: Commit): void {
    this.#hoverActivated = !this.#hoverActivated;
    commit.selected = true;
    if (!this.#hoverActivated) {
      const firstSelected = this.#commits.findIndex((x) => x.selected);
      const lastSelected = this.#commits.findLastIndex((x) => x.selected);
      const beforeCommitID = firstSelected === 0 ? this.#mergeBase : this.#commits[firstSelected - 1].id;
      const afterCommitID = this.#commits[lastSelected].id;

      if (firstSelected === lastSelected) {
        window.location.assign(`${this.#issueLink}/commits/${afterCommitID}${this.#queryParams}`);
      } else if (beforeCommitID === this.#mergeBase && afterCommitID === this.#commits.at(-1)!.id) {
        window.location.assign(`${this.#issueLink}/files${this.#queryParams}`);
      } else {
        window.location.assign(`${this.#issueLink}/files/${beforeCommitID}..${afterCommitID}${this.#queryParams}`);
      }
    }
    this.#renderMenu();
  }

  #highlight(commit: Commit): void {
    if (!this.#hoverActivated) return;
    const indexSelected = this.#commits.findIndex((x) => x.selected);
    const indexCurrentElem = this.#commits.findIndex((x) => x.id === commit.id);
    for (const [idx, c] of this.#commits.entries()) {
      c.hovered = Math.min(indexSelected, indexCurrentElem) <= idx && idx <= Math.max(indexSelected, indexCurrentElem);
    }
    this.#renderMenu();
  }

  #render(): void {
    const styles = `
      <style>
        .diff-commit-selector { position: relative; display: inline-block; }
        .diff-commit-selector .menu {
          position: absolute;
          top: 100%;
          left: 0;
          margin-top: 0.25em;
          padding: 0;
          overflow-x: hidden;
          max-height: 450px;
          background: var(--color-box-body);
          border: 1px solid var(--color-secondary);
          border-radius: var(--border-radius);
          box-shadow: var(--shadow-lg);
          z-index: 100;
          min-width: 300px;
        }
        .diff-commit-selector .menu.hidden { display: none; }
        .diff-commit-selector .loading-indicator {
          height: 200px; width: 350px;
          background: linear-gradient(90deg, var(--color-secondary-alpha-60) 25%, var(--color-secondary-alpha-30) 50%, var(--color-secondary-alpha-60) 75%);
          background-size: 200% 100%;
          animation: loading 1.5s infinite;
        }
        @keyframes loading { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }
        .diff-commit-selector .menu > .item,
        .diff-commit-selector .menu > .info {
          display: flex;
          flex-direction: row;
          line-height: 1.4;
          gap: 0.25em;
          width: auto;
          margin: 0;
          padding: 7px 14px;
          border-radius: 0;
          cursor: pointer;
        }
        .diff-commit-selector .menu > .item:not(:first-child),
        .diff-commit-selector .menu > .info:not(:first-child) {
          border-top: 1px solid var(--color-secondary);
        }
        .diff-commit-selector .menu > .item:focus {
          background: var(--color-active);
          outline: none;
        }
        .diff-commit-selector .menu > .item.hovered {
          background-color: var(--color-small-accent);
        }
        .diff-commit-selector .menu > .item.selected {
          background-color: var(--color-accent);
        }
        .diff-commit-selector .menu > .item.disabled {
          opacity: 0.5;
          cursor: not-allowed;
        }
        .diff-commit-selector .commit-list-summary {
          max-width: min(380px, 96vw);
        }
        .diff-commit-selector .expand-btn { padding: 0.25rem 0.5rem; }
      </style>
    `;

    const template = `
      <div class="diff-commit-selector">
        <button
          class="ui tiny basic button expand-btn"
          data-tooltip-content="${this.#locale.filter_changes_by_commit}"
          aria-haspopup="true"
          aria-label="${this.#locale.filter_changes_by_commit}"
          aria-controls="${this.#uniqueIdMenu}"
          aria-activedescendant="${this.#uniqueIdShowAll}"
        >
          <svg-icon name="octicon-git-commit"></svg-icon>
        </button>
        <div
          class="left menu transition ${this.#menuVisible ? '' : 'hidden'}"
          id="${this.#uniqueIdMenu}"
          role="menu"
          aria-expanded="${this.#menuVisible ? 'true' : 'false'}"
        >
          ${this.#isLoading ? `<div class="loading-indicator is-loading"></div>` : ''}
          ${!this.#isLoading ? `
            <div class="item show-all-changes" id="${this.#uniqueIdShowAll}" role="menuitem" tabindex="0">
              <div class="gt-ellipsis">${this.#locale.show_all_commits || 'Show all commits'}</div>
              <div class="gt-ellipsis tw-text-text-light-2 tw-mb-0">${this.#locale.stats_num_commits || ''}</div>
            </div>
            ${this.#lastReviewCommitSha ? `
              <div class="item ${!this.#commitsSinceLastReview ? 'disabled' : ''}" role="menuitem" tabindex="0" ${!this.#commitsSinceLastReview ? 'aria-disabled="true"' : ''}>
                <div class="gt-ellipsis">${this.#locale.show_changes_since_your_last_review || 'Show changes since last review'}</div>
                <div class="gt-ellipsis tw-text-text-light-2">${this.#commitsSinceLastReview} commits</div>
              </div>
            ` : ''}
            <span class="info tw-text-text-light-2">${this.#locale.select_commit_hold_shift_for_range || 'Select commit (hold Shift for range)'}</span>
            ${this.#commits.map((commit, idx) => `
              <div
                class="item ${commit.selected ? 'selected' : ''} ${commit.hovered ? 'hovered' : ''}"
                role="menuitem"
                tabindex="0"
                data-commit-idx="${idx}"
                data-commit-id="${commit.id}"
              >
                <div class="tw-flex-1 tw-flex tw-flex-col tw-gap-1">
                  <div class="gt-ellipsis commit-list-summary">${commit.summary}</div>
                  <div class="gt-ellipsis tw-text-text-light-2">
                    ${commit.committer_or_author_name}
                    <span class="text right">
                      <relative-time prefix="" datetime="${commit.time}" data-tooltip-content data-tooltip-interactive="true">${commit.time}</relative-time>
                    </span>
                  </div>
                </div>
                <div class="tw-font-mono">${commit.short_sha}</div>
              </div>
            `).join('')}
          ` : ''}
        </div>
      </div>
    `;

    this.shadowRoot!.innerHTML = styles + template;

    // Re-query elements after render
    this.#elRoot = this.shadowRoot!.querySelector('.diff-commit-selector')!;
    this.#elExpandBtn = this.shadowRoot!.querySelector('.expand-btn')!;
    this.#elShowAllChanges = this.shadowRoot!.querySelector('.show-all-changes')!;

    // Add event listeners
    this.#elExpandBtn.addEventListener('click', (e) => { e.stopPropagation(); this.#toggleMenu() });

    const showAllEl = this.shadowRoot!.querySelector('.show-all-changes');
    showAllEl?.addEventListener('click', () => this.#showAllChanges());
    showAllEl?.addEventListener('keydown', ((e: KeyboardEvent) => { if (e.key === 'Enter') this.#showAllChanges(); }) as EventListener);

    if (this.#lastReviewCommitSha && this.#commitsSinceLastReview > 0) {
      const reviewEl = this.shadowRoot!.querySelector('.item:not(.show-all-changes):not(.info)');
      reviewEl?.addEventListener('click', () => this.#changesSinceLastReviewClick());
      reviewEl?.addEventListener('keydown', ((e: KeyboardEvent) => { if (e.key === 'Enter') this.#changesSinceLastReviewClick(); }) as EventListener);
    }

    // Commit items
    for (const item of this.shadowRoot!.querySelectorAll('.item[data-commit-idx]')) {
      const commitIdx = Number(item.getAttribute('data-commit-idx'));
      const commitId = item.getAttribute('data-commit-id')!;

      item.addEventListener('click', ((e: MouseEvent) => {
        if (e.ctrlKey || e.metaKey) this.#commitClicked(commitId, true);
        else if (e.shiftKey) { e.stopPropagation(); e.preventDefault(); this.#commitClickedShift(this.#commits[commitIdx]) } else this.#commitClicked(commitId);
      }) as EventListener);
      item.addEventListener('keydown', ((e: KeyboardEvent) => {
        if (e.key === 'Enter') {
          if (e.shiftKey) { e.preventDefault(); this.#commitClickedShift(this.#commits[commitIdx]) } else this.#commitClicked(commitId);
        }
      }) as EventListener);
      item.addEventListener('mouseover', ((e: MouseEvent) => {
        if (e.shiftKey) this.#highlight(this.#commits[commitIdx]);
      }) as EventListener);
    }
  }

  #renderMenu(): void {
    const menu = this.shadowRoot!.querySelector('.menu');
    if (!menu) return;

    const wasVisible = this.#menuVisible;
    const expandBtn = this.shadowRoot!.querySelector('.expand-btn') as HTMLButtonElement;

    menu.classList.toggle('hidden', !this.#menuVisible);
    expandBtn.setAttribute('aria-expanded', this.#menuVisible ? 'true' : 'false');
    menu.setAttribute('aria-expanded', this.#menuVisible ? 'true' : 'false');

    if (this.#menuVisible && !wasVisible) {
      // Menu just opened, focus first item
      queueMicrotask(() => this.#focusElem(this.#elShowAllChanges, this.#elExpandBtn));
    } else if (!this.#menuVisible && wasVisible) {
      queueMicrotask(() => this.#focusElem(this.#elExpandBtn, this.#elShowAllChanges));
    }
  }
}

if (!customElements.get('diff-commit-selector')) {
  customElements.define('diff-commit-selector', DiffCommitSelector);
}
