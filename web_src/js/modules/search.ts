import {debounce} from 'throttle-debounce';
import {GET} from './fetch.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {urlQueryEscape} from '../utils/url.ts';

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

function buildResultElement(result: SearchResult): HTMLElement {
  const item = document.createElement('div');
  item.className = 'result';
  item.innerHTML = buildResultHTML(result);
  return item;
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
  const itemResults = new Map<HTMLElement, SearchResult>();
  let fetchController: AbortController | null = null;
  let search: ReturnType<typeof debounce<(query: string) => Promise<void>>> | null = null;

  const hide = () => {
    search?.cancel();
    fetchController?.abort();
    resultsEl.style.display = 'none';
    resultsEl.replaceChildren();
    itemResults.clear();
  };

  const render = (results: SearchResult[]) => {
    if (!results.length) return hide();
    itemResults.clear();
    resultsEl.replaceChildren(...results.map((result) => {
      const item = buildResultElement(result);
      itemResults.set(item, result);
      return item;
    }));
    resultsEl.style.display = 'block';
  };

  const select = (item: HTMLElement) => {
    input.value = itemResults.get(item)!.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
    hide();
  };

  search = debounce(200, async (query: string) => {
    fetchController?.abort();
    if (query.length < minCharacters) return hide();
    const ctrl = (fetchController = new AbortController());
    try {
      const response = await GET(url.replaceAll('{query}', urlQueryEscape(query)), {signal: ctrl.signal});
      if (!response.ok) return hide();
      const results = parse(await response.json(), query);
      // hide() ran (signal aborted) or a newer keystroke landed before the response did
      if (!ctrl.signal.aborted && input.value === query) render(results);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') hide();
    }
  });

  input.addEventListener('input', () => search(input.value));
  input.addEventListener('focus', () => { if (itemResults.size) resultsEl.style.display = 'block'; });
  input.addEventListener('blur', () => setTimeout(hide, 150)); // deferred so a result mousedown can land first
  input.addEventListener('keydown', (event) => {
    const all = Array.from(resultsEl.querySelectorAll<HTMLElement>('.result'));
    if (!all.length) return;
    const index = all.findIndex((item) => item.classList.contains('active'));
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      all[index]?.classList.remove('active');
      const next = event.key === 'ArrowDown' ? (index + 1) % all.length : index <= 0 ? all.length - 1 : index - 1;
      all[next].classList.add('active');
    } else if (event.key === 'Enter' && index >= 0) {
      event.preventDefault();
      select(all[index]);
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
}
