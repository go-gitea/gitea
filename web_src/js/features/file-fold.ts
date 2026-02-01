import {svg} from '../svg.ts';

function parseTransitionValue(value: string): number {
  let max = 0;
  for (const current of value.split(',')) {
    const trimmed = current.trim();
    if (!trimmed) continue;
    const isMs = trimmed.endsWith('ms');
    const numericPortion = Number.parseFloat(trimmed.replace(/ms|s$/u, ''));
    if (Number.isNaN(numericPortion)) continue;
    const duration = numericPortion * (isMs ? 1 : 1000);
    max = Math.max(max, duration);
  }
  return max;
}

function waitForTransitionEnd(element: Element): Promise<void> {
  if (!(element instanceof HTMLElement)) return Promise.resolve();
  const transitionTarget = element.querySelector<HTMLElement>('.diff-file-body') ?? element;
  const styles = window.getComputedStyle(transitionTarget);
  const transitionDuration = parseTransitionValue(styles.transitionDuration);
  const transitionDelay = parseTransitionValue(styles.transitionDelay);
  const total = transitionDuration + transitionDelay;
  if (total === 0) return Promise.resolve();

  return new Promise((resolve) => {
    let resolved = false;
    function cleanup() {
      if (resolved) return;
      resolved = true;
      transitionTarget.removeEventListener('transitionend', onTransitionEnd);
      resolve();
    }
    function onTransitionEnd(event: TransitionEvent) {
      if (event.target !== transitionTarget) return;
      cleanup();
    }
    transitionTarget.addEventListener('transitionend', onTransitionEnd);
    window.setTimeout(cleanup, total + 50);
  });
}

// Hides the file if newFold is true, and shows it otherwise. The actual hiding is performed using CSS.
//
// The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
// The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
//
export function setFileFolding(fileContentBox: Element, foldArrow: HTMLElement, newFold: boolean): Promise<void> {
  foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
  fileContentBox.setAttribute('data-folded', String(newFold));
  if (newFold && fileContentBox.getBoundingClientRect().top < 0) {
    fileContentBox.scrollIntoView();
  }
  return waitForTransitionEnd(fileContentBox);
}

// Like `setFileFolding`, except that it automatically inverts the current file folding state.
export function invertFileFolding(fileContentBox:HTMLElement, foldArrow: HTMLElement): Promise<void> {
  return setFileFolding(fileContentBox, foldArrow, fileContentBox.getAttribute('data-folded') !== 'true');
}
