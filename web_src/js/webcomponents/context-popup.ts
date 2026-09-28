// ContextPopup Web Component
// Simple issue preview popup - converted from Vue component

type IssueData = {
  repository: {
    full_name: string;
  };
  created_at: string;
  body: string;
  html_url: string;
  title: string;
  number: number;
  labels: Array<{name: string; color: string; url: string}>;
};

type ContextPopupLocale = {
  errorMessage?: string;
};

class ContextPopup extends HTMLElement {
  #issue: IssueData | null = null;
  #renderedLabels = '';
  #errorMessage = '';
  #locale: ContextPopupLocale = {};

  static get observedAttributes(): string[] {
    return ['data-issue', 'data-rendered-labels', 'data-error-message', 'data-locale'];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    if (name === 'data-issue' && newVal) {
      try {
        this.#issue = JSON.parse(newVal);
      } catch {
        this.#issue = null;
      }
    } else if (name === 'data-rendered-labels') {
      this.#renderedLabels = newVal;
    } else if (name === 'data-error-message') {
      this.#errorMessage = newVal;
    } else if (name === 'data-locale' && newVal) {
      try {
        this.#locale = JSON.parse(newVal);
      } catch {
        this.#locale = {};
      }
    }
    this.#render();
  }

  connectedCallback(): void {
    this.#render();
  }

  #render(): void {
    const appSubUrl = (window as any).config?.appSubUrl || '';

    const styles = `
      <style>
        .context-popup { padding: 1rem; max-width: 400px; }
        .context-popup .repo-name { font-size: 0.75rem; color: var(--color-text-light-2); margin-bottom: 0.5rem; }
        .context-popup .repo-name a { color: inherit; text-decoration: none; }
        .context-popup .issue-header { display: flex; align-items: flex-start; gap: 0.5rem; margin-bottom: 0.5rem; }
        .context-popup .issue-icon { flex-shrink: 0; width: 16px; height: 16px; }
        .context-popup .issue-title { font-weight: 600; word-break: break-word; color: inherit; text-decoration: none; }
        .context-popup .issue-title:hover { color: var(--color-primary); }
        .context-popup .issue-index { color: var(--color-text-light-1); }
        .context-popup .issue-body { font-size: 0.875rem; color: var(--color-text-light-1); margin-bottom: 0.5rem; }
        .context-popup .issue-labels { display: flex; flex-wrap: wrap; gap: 0.25rem; }
        .context-popup .error-message { color: var(--color-error-text); padding: 1rem; text-align: center; }
      </style>
    `;

    let content = '';

    let createdAt = '';
    if (this.#issue) {
      createdAt = new Date(this.#issue.created_at).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      });

      const body = this.#issue.body.replace(/\n+/g, ' ');
      const truncatedBody = body.length > 85 ? `${body.substring(0, 85)}…` : body;

      const issueIcon = this.#getIssueIcon(this.#issue);
      const issueColorClass = this.#getIssueColorClass(this.#issue);

      content = `
        <div class="issue-header">
          <svg-icon name="${issueIcon}" class="issue-icon ${issueColorClass}"></svg-icon>
          <a href="${escapeHtml(this.#issue.html_url)}" class="issue-title">
            ${escapeHtml(this.#issue.title)}
            <span class="issue-index">#${this.#issue.number}</span>
          </a>
        </div>
        ${truncatedBody ? `<div class="issue-body">${escapeHtml(truncatedBody)}</div>` : ''}
        ${this.#renderedLabels ? `<div class="issue-labels">${this.#renderedLabels}</div>` : ''}
      `;
    } else if (this.#errorMessage) {
      content = `<div class="error-message">${escapeHtml(this.#errorMessage)}</div>`;
    } else {
      content = `<div class="error-message">${this.#locale.errorMessage || 'Failed to load issue'}</div>`;
    }

    const template = `
      <div class="context-popup">
        ${this.#issue ? `
          <div class="repo-name">
            <a href="${escapeHtml(`${appSubUrl}/${this.#issue.repository.full_name}`)}">${escapeHtml(this.#issue.repository.full_name)}</a>
            on ${createdAt}
          </div>
        ` : ''}
        ${content}
      </div>
    `;

    this.shadowRoot!.innerHTML = styles + template;
  }

  #getIssueIcon(_issue: IssueData): string {
    // Simplified - in the original this came from a separate module
    // Default to octicon-issue-opened for open issues
    return 'octicon-issue-opened';
  }

  #getIssueColorClass(_issue: IssueData): string {
    // Simplified - in the original this came from a separate module
    return 'tw-text-green';
  }
}

// @ts-ignore: Duplicate function implementation (false positive)
function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

if (!customElements.get('context-popup')) {
  customElements.define('context-popup', ContextPopup);
}
