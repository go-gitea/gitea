// RepoActivityTopAuthors Web Component
// Static SVG bar chart for top contributors - converted from Vue component

type AuthorData = {
  name: string;
  avatar_link: string;
  home_link: string;
  commits: number;
};

type RepoActivityTopAuthorsLocale = {
  filterLabel: string;
  contributionType: Record<string, string>;
  loadingTitle: string;
  loadingTitleFailed: string;
  loadingInfo: string;
  chartZoomHint: string;
};

class RepoActivityTopAuthors extends HTMLElement {
  #data: AuthorData[] = [];
  #locale: RepoActivityTopAuthorsLocale | null = null;
  #boundResize!: () => void;

  static get observedAttributes(): string[] {
    return ['data-authors', 'data-locale'];
  }

  attributeChangedCallback(name: string, _old: string, newVal: string): void {
    if (name === 'data-authors' && newVal) {
      try {
        this.#data = JSON.parse(newVal);
      } catch {
        this.#data = [];
      }
    } else if (name === 'data-locale' && newVal) {
      try {
        this.#locale = JSON.parse(newVal);
      } catch {
        this.#locale = null;
      }
    }
    this.#render();
  }

  connectedCallback(): void {
    this.#boundResize = this.#render.bind(this);
    window.addEventListener('resize', this.#boundResize);
    this.#render();
  }

  disconnectedCallback(): void {
    window.removeEventListener('resize', this.#boundResize);
  }

  #render(): void {
    if (!this.#locale) return;

    const barSlotWidth = 40;
    const chartHeight = 100;
    const innerChartHeight = chartHeight - 28;
    const barMidPoint = barSlotWidth / 2;
    const barWidth = barSlotWidth - 2;
    const avatarSize = 20;
    const labelInsideThreshold = 22;

    const activityTopAuthors = this.#data;
    const graphWidth = activityTopAuthors.length * barSlotWidth;
    const maxCommits = Math.max(...activityTopAuthors.map((author) => author.commits), 1);

    const bars = activityTopAuthors.map((author, index) => {
      const height = (author.commits / maxCommits) * innerChartHeight;
      return {
        author,
        index,
        x: index * barSlotWidth,
        height,
        yOffset: innerChartHeight - height,
        labelInside: height >= labelInsideThreshold,
      };
    });

    // Compute colors from CSS custom properties
    const style = document.createElement('div');
    style.className = 'activity-bar-graph';
    style.style.cssText = 'width:0;height:0;position:absolute;visibility:hidden;';
    document.body.append(style);
    const refStyle = window.getComputedStyle(style);
    const altStyle = document.createElement('div');
    altStyle.className = 'activity-bar-graph-alt';
    altStyle.style.cssText = 'width:0;height:0;position:absolute;visibility:hidden;';
    document.body.append(altStyle);
    const refAltStyle = window.getComputedStyle(altStyle);
    style.remove();
    altStyle.remove();

    const colors = {
      barColor: refStyle.backgroundColor || 'green',
      textColor: refStyle.color || 'black',
      textAltColor: refAltStyle.color || 'white',
    };

    const styles = `
      <style>
        .activity-chart { display: flex; flex-direction: column; }
        .activity-chart svg { display: block; width: 100%; height: auto; }
        .axis-line { stroke: var(--color-secondary-alpha-60); stroke-width: 1; }
      </style>
    `;

    const svg = `
      <svg width="${graphWidth}" height="${chartHeight}" xmlns="http://www.w3.org/2000/svg">
        ${bars.map((bar) => `
          <g transform="translate(${bar.x},0)">
            <title>${escapeHtml(bar.author.name)}</title>
            <rect x="2" y="${bar.yOffset}" width="${barWidth}" height="${bar.height}" style="fill: ${colors.barColor}"/>
            <text x="${barMidPoint}" y="${bar.yOffset}" dy="${bar.labelInside ? '15px' : '-5px'}" text-anchor="middle" style="fill: ${bar.labelInside ? colors.textAltColor : colors.textColor}; font: 10px sans-serif;">${bar.author.commits}</text>
            ${bar.author.home_link ? `
              <a href="${escapeHtml(bar.author.home_link)}">
                <image x="${barMidPoint - avatarSize / 2}" y="${innerChartHeight + 4}" height="${avatarSize}" width="${avatarSize}" href="${escapeHtml(bar.author.avatar_link)}"/>
              </a>
            ` : `
              <image x="${barMidPoint - avatarSize / 2}" y="${innerChartHeight + 4}" height="${avatarSize}" width="${avatarSize}" href="${escapeHtml(bar.author.avatar_link)}"/>
            `}
            <line class="axis-line" x1="${barMidPoint}" x2="${barMidPoint}" y1="${innerChartHeight + 3}" y2="${innerChartHeight}"/>
          </g>
        `).join('')}
        <line class="axis-line" x1="2" x2="${graphWidth}" y1="${innerChartHeight}" y2="${innerChartHeight}"/>
      </svg>
    `;

    this.shadowRoot!.innerHTML = styles + svg;
  }
}

// @ts-ignore: Duplicate function implementation (false positive)
function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

if (!customElements.get('repo-activity-top-authors')) {
  customElements.define('repo-activity-top-authors', RepoActivityTopAuthors);
}
