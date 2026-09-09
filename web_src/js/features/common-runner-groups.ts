import {registerGlobalInitFunc} from '../modules/observer.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRunnerGroupsInput(): void {
  registerGlobalInitFunc('initRunnerGroupsInput', (el: HTMLElement) => {
    const $dropdown = fomanticQuery(el);
    $dropdown.dropdown({allowAdditions: true, forceSelection: false});

    // fomantic does not commit an addition when the menu is empty, which is every instance with no groups yet
    const search = el.querySelector<HTMLInputElement>('input.search')!;
    search.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key !== 'Enter' && e.key !== ',') return;
      e.preventDefault();
      e.stopPropagation(); // fomantic would otherwise commit a second, highlighted item
      // the menu highlight lags behind typing by settings.delay.search, so typed text wins
      const highlighted = el.querySelector<HTMLElement>('.menu .item.selected:not(.filtered)');
      const value = search.value.trim() || highlighted?.getAttribute('data-value') || '';
      if (!value) return;
      $dropdown.dropdown('set selected', value);
      $dropdown.dropdown('remove search term');
    });
  });
}
