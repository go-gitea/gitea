import {Virtualizer, measureElement, observeWindowOffset, observeWindowRect, windowScroll} from '@tanstack/virtual-core';
import {createElementFromAttrs} from '../../utils/dom.ts';

const stickyTopHeight = 44; // matches .diff-file-box scroll-margin-top

type VirtualListOptions<T> = {
  key: (item: T) => string; // sizes are cached by key, so a filter change keeps the measured heights
  estimateHeight: (item: T) => number;
  element: (item: T) => HTMLElement; // the caller keeps the elements of unmounted items
};

// a window-scrolled list which only mounts the elements of the items in view, positioned absolutely in a box of the total height
export class VirtualList<T extends object> {
  readonly element = createElementFromAttrs('div', {class: 'tw-relative'});
  items: T[] = [];
  #root: HTMLElement;
  #opts: VirtualListOptions<T>;
  #virtualizer: Virtualizer<Window, HTMLElement> | null = null;
  #mounted = new Map<T, HTMLElement>();
  #itemsByElement = new WeakMap<HTMLElement, T>();
  #mountQueued = false;

  constructor(root: HTMLElement, opts: VirtualListOptions<T>) {
    this.#root = root;
    this.#opts = opts;
  }

  get started(): boolean {
    return this.#virtualizer !== null;
  }

  itemOf(el: HTMLElement): T | undefined {
    return this.#itemsByElement.get(el);
  }

  // appends the list to the root and mounts the items in view, later calls do nothing
  start(): void {
    if (this.#virtualizer) return;
    this.#root.append(this.element);
    this.#virtualizer = new Virtualizer<Window, HTMLElement>({
      count: this.items.length,
      getScrollElement: () => window,
      estimateSize: (index) => this.#opts.estimateHeight(this.items[index]),
      getItemKey: (index) => this.#opts.key(this.items[index]),
      observeElementRect: observeWindowRect,
      observeElementOffset: observeWindowOffset,
      scrollToFn: windowScroll,
      scrollMargin: this.#scrollMargin(),
      scrollPaddingStart: stickyTopHeight,
      // the synchronous measurement on mount would force a layout per item, the ResizeObserver delivers the size after the regular one
      measureElement: (el, entry, instance) => {
        if (entry) return measureElement(el, entry, instance);
        return instance.itemSizeCache.get(instance.options.getItemKey(instance.indexFromElement(el))) ?? this.#opts.estimateHeight(this.itemOf(el)!);
      },
      overscan: 3,
      gap: 8,
      onChange: () => this.queueMount(),
    });
    this.#virtualizer._didMount();
    this.#virtualizer._willUpdate();
  }

  setItems(items: T[]): void {
    this.items = items;
    if (!this.#virtualizer) return;
    this.#virtualizer.setOptions({...this.#virtualizer.options, count: items.length});
    this.queueMount();
  }

  // when the content above the list changes its height
  updateScrollMargin(): void {
    if (!this.#virtualizer) return;
    this.#virtualizer.setOptions({...this.#virtualizer.options, scrollMargin: this.#scrollMargin()});
    this.queueMount();
  }

  // measures the mounted elements again, after their heights changed at once
  measure(): void {
    this.#virtualizer?.measure();
  }

  scrollToIndex(index: number): void {
    this.#virtualizer?.scrollToIndex(index, {align: 'start'});
  }

  // removes the element of an item, it is mounted again from the element option
  unmount(item: T): void {
    this.#mounted.get(item)?.remove();
    this.#mounted.delete(item);
    this.queueMount();
  }

  queueMount(): void {
    if (this.#mountQueued) return;
    this.#mountQueued = true;
    queueMicrotask(() => {
      this.#mountQueued = false;
      this.#mount();
    });
  }

  #scrollMargin(): number {
    return this.element.getBoundingClientRect().top + window.scrollY;
  }

  #mount(): void {
    const virtualizer = this.#virtualizer;
    if (!virtualizer) return;
    const virtualItems = virtualizer.getVirtualItems();
    this.element.style.height = `${virtualizer.getTotalSize()}px`;
    const wanted = new Set(virtualItems.map((virtualItem) => this.items[virtualItem.index]));
    for (const [item, el] of this.#mounted) {
      if (!wanted.has(item)) {
        el.remove();
        this.#mounted.delete(item);
      }
    }
    for (const virtualItem of virtualItems) {
      const item = this.items[virtualItem.index];
      const el = this.#mounted.get(item) ?? this.#opts.element(item);
      el.setAttribute('data-index', String(virtualItem.index));
      el.style.top = `${virtualItem.start - virtualizer.options.scrollMargin}px`;
      if (!this.#mounted.has(item)) {
        this.#itemsByElement.set(el, item);
        this.#mounted.set(item, el);
        this.element.append(el);
        virtualizer.measureElement(el);
      }
    }
  }
}
