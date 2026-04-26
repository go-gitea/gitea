import {debounce} from 'throttle-debounce';
import {GET} from './fetch.ts';
import {html, htmlRaw} from '../utils/html.ts';

export type SearchResult = {
  title: string;
  description?: string;
  image?: string;
};

function buildResultHTML(result: SearchResult): string {
  const img = result.image ? html`<div class="image"><img src="${result.image}" alt=""></div>` : '';
  const desc = result.description ? html`<div class="description">${result.description}</div>` : '';
  return html`${htmlRaw(img)}<div class="content"><div class="title">${result.title}</div>${htmlRaw(desc)}</div>`;
}

/** Attach an API-driven autocomplete to `container`. `parse` maps the raw JSON response into the rendered result list. The selected result's title is written to the input on selection. */
export function attachSearchBox<T = unknown>(container: HTMLElement, url: string, parse: (raw: T, query: string) => SearchResult[], {minCharacters = 2}: {minCharacters?: number} = {}): void {
  const input = container.querySelector<HTMLInputElement>('input.prompt') ?? container.querySelector<HTMLInputElement>('input');
  if (!input) return;

  let resultsEl = container.querySelector<HTMLElement>(':scope > .results');
  if (!resultsEl) {
    resultsEl = document.createElement('div');
    resultsEl.className = 'results';
    container.append(resultsEl);
  }

  let fetchController: AbortController | null = null;
  const itemResults = new Map<HTMLElement, SearchResult>();
  const items = () => resultsEl.querySelectorAll<HTMLElement>('.result');

  const hide = () => {
    resultsEl.style.display = 'none';
    resultsEl.replaceChildren();
    itemResults.clear();
  };

  const render = (results: SearchResult[]) => {
    if (!results.length) return hide();
    resultsEl.replaceChildren();
    itemResults.clear();
    for (const result of results) {
      const item = document.createElement('div');
      item.className = 'result';
      item.innerHTML = buildResultHTML(result);
      itemResults.set(item, result);
      resultsEl.append(item);
    }
    resultsEl.style.display = 'block';
  };

  const select = (item: HTMLElement) => {
    const picked = itemResults.get(item)!;
    input.value = picked.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
    hide();
  };

  const performSearch = async (query: string) => {
    fetchController?.abort();
    if (query.length < minCharacters) return hide();
    fetchController = new AbortController();
    try {
      const response = await GET(url.replaceAll('{query}', encodeURIComponent(query)), {signal: fetchController.signal});
      if (!response.ok) return hide();
      const results = parse(await response.json(), query);
      if (input.value !== query) return; // stale response racing a newer keystroke
      render(results);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') hide();
    }
  };

  const debounced = debounce(200, (query: string) => { performSearch(query) });

  input.addEventListener('input', () => debounced(input.value));
  input.addEventListener('focus', () => { if (items().length) resultsEl.style.display = 'block'; });
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
      select(all[activeIndex]);
    } else if (event.key === 'Escape') {
      hide();
    }
  });
  // mousedown fires before input blur so the selection registers before blur-hide kicks in
  resultsEl.addEventListener('mousedown', (event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('.result');
    if (!target) return;
    event.preventDefault();
    select(target);
  });
  document.addEventListener('click', (event) => {
    if (!container.contains(event.target as Node)) hide();
  });
  input.addEventListener('blur', () => setTimeout(hide, 150)); // deferred so a result mousedown can land first
}
