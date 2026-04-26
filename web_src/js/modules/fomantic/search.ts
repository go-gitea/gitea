import {debounce} from 'throttle-debounce';
import {GET} from '../fetch.ts';
import {html, htmlRaw} from '../../utils/html.ts';

export type SearchResult = {
  title: string;
  description?: string;
  image?: string;
};

export type SearchOpts = {
  apiUrl: string;
  minCharacters?: number;
  onResponse: (raw: any, query: string) => {results: SearchResult[]};
};

function buildResultHTML(result: SearchResult): string {
  const img = result.image ? html`<div class="image"><img src="${result.image}" alt=""></div>` : '';
  const desc = result.description ? html`<div class="description">${htmlRaw(result.description)}</div>` : '';
  return html`${htmlRaw(img)}<div class="content"><div class="title">${htmlRaw(result.title)}</div>${htmlRaw(desc)}</div>`;
}

export function initSearchBox(container: HTMLElement, opts: SearchOpts): void {
  const minCharacters = opts.minCharacters ?? 2;
  const input = container.querySelector<HTMLInputElement>('input.prompt') ?? container.querySelector<HTMLInputElement>('input');
  if (!input) return;

  let resultsEl = container.querySelector<HTMLElement>(':scope > .results');
  if (!resultsEl) {
    resultsEl = document.createElement('div');
    resultsEl.className = 'results';
    container.append(resultsEl);
  }

  let abortCtrl: AbortController | null = null;

  const items = () => resultsEl.querySelectorAll<HTMLElement>('.result');

  const hide = () => {
    resultsEl.style.display = 'none';
    resultsEl.replaceChildren();
  };

  const render = (results: SearchResult[]) => {
    if (!results.length) return hide();
    resultsEl.replaceChildren();
    for (const result of results) {
      const item = document.createElement('div');
      item.className = 'result';
      item.innerHTML = buildResultHTML(result);
      resultsEl.append(item);
    }
    resultsEl.style.display = 'block';
  };

  const selectItem = (item: HTMLElement) => {
    input.value = item.querySelector<HTMLElement>('.title')!.textContent ?? '';
    input.dispatchEvent(new Event('change', {bubbles: true}));
    hide();
    input.focus();
  };

  const performSearch = async (query: string) => {
    abortCtrl?.abort();
    if (query.length < minCharacters) return hide();
    abortCtrl = new AbortController();
    try {
      const response = await GET(opts.apiUrl.replaceAll('{query}', encodeURIComponent(query)), {signal: abortCtrl.signal});
      if (!response.ok) return hide();
      const {results} = opts.onResponse(await response.json(), query);
      if (input.value !== query) return; // stale response racing a newer keystroke
      render(results);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') hide();
    }
  };

  const debounced = debounce(200, (query: string) => { performSearch(query) });
  input.addEventListener('input', () => debounced(input.value));
  input.addEventListener('focus', () => {
    if (items().length) resultsEl.style.display = 'block';
  });

  input.addEventListener('keydown', (event) => {
    const all = items();
    if (!all.length) return;
    const activeIndex = Array.from(all).findIndex((item) => item.classList.contains('active'));
    const move = (next: number) => {
      event.preventDefault();
      all[activeIndex]?.classList.remove('active');
      all[next].classList.add('active');
    };
    if (event.key === 'ArrowDown') {
      move((activeIndex + 1) % all.length);
    } else if (event.key === 'ArrowUp') {
      move(activeIndex <= 0 ? all.length - 1 : activeIndex - 1);
    } else if (event.key === 'Enter' && activeIndex >= 0) {
      event.preventDefault();
      selectItem(all[activeIndex]);
    } else if (event.key === 'Escape') {
      hide();
    }
  });

  // mousedown fires before input blur so the selection registers before blur-hide kicks in
  resultsEl.addEventListener('mousedown', (event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('.result');
    if (!target) return;
    event.preventDefault();
    selectItem(target);
  });

  document.addEventListener('click', (event) => {
    if (!container.contains(event.target as Node)) hide();
  });

  input.addEventListener('blur', () => {
    setTimeout(hide, 150); // deferred so a result mousedown can land first
  });
}
