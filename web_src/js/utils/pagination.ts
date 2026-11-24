// A page number, or null where skipped pages are collapsed into an ellipsis.
export type PaginationPage = number | null;

// Five numbers plus the two ellipses match the server side navigation.
const numPages = 5;

// getPaginationPages mirrors modules/paginator.Paginator.Pages so the sidebar
// pages the same way as every other list: numbers centred on the current page,
// extended to one side near either end.
export function getPaginationPages(current: number, total: number): PaginationPage[] {
  if (total <= 0) return [];
  if (total <= numPages) return Array.from({length: total}, (_, i) => i + 1);

  current = Math.min(Math.max(current, 1), total);

  let previous = Math.min(Math.floor((numPages + 1) / 2) - 1, current - 1);
  let next = numPages - previous - 1;
  if (current + next > total) {
    const delta = next - (total - current);
    next -= delta;
    previous += delta;
  }

  const first = current - previous;
  const pages: PaginationPage[] = [];
  if (first > 1) pages.push(null);
  for (let i = 0; i < previous; i++) pages.push(first + i);
  pages.push(current);
  for (let i = 1; i <= next; i++) pages.push(current + i);
  if (current + next < total) pages.push(null);
  return pages;
}
