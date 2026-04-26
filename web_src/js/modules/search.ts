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

// Awaits one user selection from an autocomplete attached to `container`. Resolves with
// the chosen item; the caller writes it to whatever input/state it owns. Wrap in a loop
// to keep the search box live across selections.
export function chooseFromApi<T = unknown>(container: HTMLElement, url: string, parse: (raw: T, query: string) => SearchResult[], {minCharacters = 2}: {minCharacters?: number} = {}): Promise<SearchResult> {
  return new Promise((resolve) => {
    const input = container.querySelector<HTMLInputElement>('input.prompt') ?? container.querySelector<HTMLInputElement>('input')!;

    let resultsEl = container.querySelector<HTMLElement>(':scope > .results');
    if (!resultsEl) {
      resultsEl = document.createElement('div');
      resultsEl.className = 'results';
      container.append(resultsEl);
    }

    let fetchController: AbortController | null = null;
    const lifecycleController = new AbortController();
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

    const finish = (item: HTMLElement) => {
      const picked = itemResults.get(item)!;
      hide();
      lifecycleController.abort();
      resolve(picked);
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

    input.addEventListener('input', () => debounced(input.value), {signal: lifecycleController.signal});
    input.addEventListener('focus', () => { if (items().length) resultsEl.style.display = 'block'; }, {signal: lifecycleController.signal});
    input.addEventListener('keydown', (event) => {
      const all = items();
      if (!all.length) return;
      const activeIndex = Array.from(all).findIndex((it) => it.classList.contains('active'));
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
        finish(all[activeIndex]);
      } else if (event.key === 'Escape') {
        hide();
      }
    }, {signal: lifecycleController.signal});
    // mousedown fires before input blur so the selection registers before blur-hide kicks in
    resultsEl.addEventListener('mousedown', (event) => {
      const target = (event.target as HTMLElement).closest<HTMLElement>('.result');
      if (!target) return;
      event.preventDefault();
      finish(target);
    }, {signal: lifecycleController.signal});
    document.addEventListener('click', (event) => {
      if (!container.contains(event.target as Node)) hide();
    }, {signal: lifecycleController.signal});
    input.addEventListener('blur', () => setTimeout(hide, 150), {signal: lifecycleController.signal}); // deferred so a result mousedown can land first
  });
}
