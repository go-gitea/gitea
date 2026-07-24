import {debounce} from 'throttle-debounce';
import {GET} from './fetch.ts';
import {errorName} from './errors.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {urlQueryEscape} from '../utils/url.ts';
import {svg} from '../svg.ts';

export type SearchResult = {
  title: string;
  description?: string;
  image?: string;
  icon?: string; // raw SVG html, rendered before the content
  link?: string;
  active?: boolean; // when defined, reserve a leading column and show a check on the active result
};

function buildResultHTML(result: SearchResult): string {
  const check = result.active === undefined ? '' : html`<div class="result-check">${htmlRaw(result.active ? svg('octicon-check', 16) : '')}</div>`;
  const icon = result.icon ? html`<div class="result-icon">${htmlRaw(result.icon)}</div>` : '';
  const img = result.image ? html`<div class="image"><img src="${result.image}" alt=""></div>` : '';
  const desc = result.description ? html`<div class="description">${result.description}</div>` : '';
  return html`${htmlRaw(check)}${htmlRaw(icon)}${htmlRaw(img)}<div class="content"><div class="title">${result.title}</div>${htmlRaw(desc)}</div>`;
}

function buildResultElement(result: SearchResult): HTMLElement {
  const item = document.createElement('div');
  item.className = 'result';
  item.innerHTML = buildResultHTML(result);
  return item;
}

// single delegated outside-click handler; each attachSearchBox registers a {container, hide} entry
const outsideClickBoxes = new Set<{container: HTMLElement; hide: () => void}>();
document.addEventListener('click', (event) => {
  for (const box of outsideClickBoxes) {
    if (!box.container.contains(event.target as Node)) box.hide();
  }
});

export type SearchBox = {
  /** run a fetch+render immediately for the current input value, bypassing the debounce */
  refresh: () => void;
};

/** Attach an API-driven autocomplete to `container`. `parse` maps the raw JSON response into the rendered result list. The selected result's title is written to the input on selection. */
export function attachSearchBox<T = unknown>(container: HTMLElement, url: string, parse: (raw: T, query: string) => SearchResult[], {minCharacters = 2, onSelect}: {minCharacters?: number; onSelect?: (result: SearchResult) => void} = {}): SearchBox {
  const input = container.querySelector<HTMLInputElement>('input.prompt') ?? container.querySelector<HTMLInputElement>('input');
  if (!input) return {refresh: () => {}};

  let resultsEl = container.querySelector<HTMLElement>(':scope > .results');
  if (!resultsEl) {
    resultsEl = document.createElement('div');
    resultsEl.className = 'results';
    container.append(resultsEl);
  }
  const itemResults = new Map<HTMLElement, SearchResult>();
  let fetchController: AbortController | null = null;
  let blurHideTimer = 0;

  const hide = () => {
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
    const result = itemResults.get(item)!;
    if (onSelect) {
      onSelect(result);
    } else {
      input.value = result.title;
      input.dispatchEvent(new Event('change', {bubbles: true}));
    }
    hide();
  };

  const doSearch = async (query: string) => {
    fetchController?.abort();
    if (query.length < minCharacters) return hide();
    const ctrl = (fetchController = new AbortController());
    try {
      const response = await GET(url.replaceAll('{query}', urlQueryEscape(query)), {signal: ctrl.signal});
      if (!response.ok) return hide();
      const results = parse(await response.json(), query);
      // only render if the fetch wasn't aborted (e.g. by hide()) and the input still matches
      if (!ctrl.signal.aborted && input.value === query) render(results);
    } catch (err) {
      if (errorName(err) !== 'AbortError') hide();
    }
  };
  const search = debounce(200, doSearch);
  // cancel + hide ensures a debounced fetch scheduled before any of these can't fire afterwards
  const dismiss = () => { search.cancel(); hide() };

  input.addEventListener('input', () => search(input.value));
  // clear any pending deferred hide so a quick refocus/reopen doesn't get its fresh results wiped
  input.addEventListener('focus', () => { clearTimeout(blurHideTimer); if (itemResults.size) resultsEl.style.display = 'block'; });
  input.addEventListener('blur', () => { search.cancel(); blurHideTimer = window.setTimeout(hide, 150) }); // hide deferred so a result mousedown can land first
  input.addEventListener('keydown', (event) => {
    const resultEls = Array.from(resultsEl.querySelectorAll<HTMLElement>('.result'));
    if (!resultEls.length) return;
    const index = resultEls.findIndex((item) => item.classList.contains('active'));
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      resultEls[index]?.classList.remove('active');
      const next = event.key === 'ArrowDown' ? (index + 1) % resultEls.length : index <= 0 ? resultEls.length - 1 : index - 1;
      resultEls[next].classList.add('active');
    } else if (event.key === 'Enter' && index >= 0) {
      event.preventDefault();
      select(resultEls[index]);
    } else if (event.key === 'Escape') {
      dismiss();
    }
  });
  // mousedown fires before input blur so the selection registers before blur-hide kicks in
  resultsEl.addEventListener('mousedown', (event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('.result');
    if (!target) return;
    event.preventDefault();
    select(target);
  });
  outsideClickBoxes.add({container, hide: dismiss});

  return {refresh: () => { search.cancel(); doSearch(input.value) }};
}
