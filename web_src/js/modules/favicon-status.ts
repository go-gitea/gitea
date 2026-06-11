import {getActionStatusIcon} from './action-status-icon.ts';
import type {ActionsStatus} from './gitea-actions.ts';
import {svgParseOuterInner} from '../svg.ts';
import {html, htmlRaw} from '../utils/html.ts';

const {svgOuter, svgInnerHtml: giteaFaviconInner} = svgParseOuterInner('gitea-favicon');
const faviconViewBox = svgOuter.getAttribute('viewBox') ?? '0 0 640 640';
const [, , faviconViewBoxWidth, faviconViewBoxHeight] = faviconViewBox.split(/\s+/).map(Number);

// Badge size follows GitHub Actions favicon proportions (~55% of the icon width).
const BADGE_ICON_SIZE = 16;
const BADGE_DRAW_SIZE = faviconViewBoxWidth * 220 / 640;
const BADGE_X = faviconViewBoxWidth - BADGE_DRAW_SIZE - 10;
const BADGE_Y = faviconViewBoxHeight - BADGE_DRAW_SIZE - 10;
const BADGE_SCALE = BADGE_DRAW_SIZE / BADGE_ICON_SIZE;

let currentStatus: ActionsStatus | null = null;
const defaultFaviconHrefs = new Map<HTMLLinkElement, string>();
const faviconDataUrlCache = new Map<ActionsStatus, string>();
let colorProbe: HTMLElement | null = null;

const TAILWIND_TEXT_COLOR_VARS: Record<string, string> = {
  'tw-text-green': '--color-green',
  'tw-text-yellow': '--color-yellow',
  'tw-text-red': '--color-red',
  'tw-text-text-light': '--color-text-light',
};

function rememberDefaultFaviconHrefs() {
  if (defaultFaviconHrefs.size > 0) return;
  for (const link of document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')) {
    defaultFaviconHrefs.set(link, link.href);
  }
}

function resolveTailwindTextColor(colorClass: string): string {
  const cssVar = TAILWIND_TEXT_COLOR_VARS[colorClass];
  if (cssVar) {
    const fromVar = getComputedStyle(document.documentElement).getPropertyValue(cssVar).trim();
    if (fromVar) return fromVar;
  }
  if (!colorProbe) {
    colorProbe = document.createElement('span');
    colorProbe.style.display = 'none';
    document.body.append(colorProbe);
  }
  colorProbe.className = colorClass;
  const fromClass = getComputedStyle(colorProbe).color;
  if (fromClass) return fromClass;
  return '#000000';
}

function buildStatusIconMarkup(status: ActionsStatus): string {
  const {name, colorClass} = getActionStatusIcon(status, 'circle-fill');
  const color = resolveTailwindTextColor(colorClass);
  const {svgInnerHtml} = svgParseOuterInner(name);
  const coloredInner = svgInnerHtml.replaceAll('currentColor', color);
  return html`<g transform="translate(${BADGE_X}, ${BADGE_Y}) scale(${BADGE_SCALE})" fill="${color}" color="${color}">${htmlRaw(coloredInner)}</g>`;
}

export function buildStatusFaviconSvg(status: ActionsStatus): string {
  return html`<svg xmlns="http://www.w3.org/2000/svg" viewBox="${faviconViewBox}">${htmlRaw(giteaFaviconInner)}${htmlRaw(buildStatusIconMarkup(status))}</svg>`;
}

function buildStatusFaviconDataUrl(status: ActionsStatus): string {
  const cached = faviconDataUrlCache.get(status);
  if (cached) return cached;
  const dataUrl = `data:image/svg+xml,${encodeURIComponent(buildStatusFaviconSvg(status))}`;
  faviconDataUrlCache.set(status, dataUrl);
  return dataUrl;
}

function setFaviconHref(href: string) {
  rememberDefaultFaviconHrefs();
  for (const link of defaultFaviconHrefs.keys()) {
    if (link.isConnected) link.href = href;
  }
}

export function syncActionRunFavicon(status: ActionsStatus | ''): void {
  if (status === '') {
    resetActionFavicon();
    return;
  }
  if (status === currentStatus) return;
  setFaviconHref(buildStatusFaviconDataUrl(status));
  currentStatus = status;
}

export function resetActionFavicon(): void {
  if (currentStatus === null) return;
  rememberDefaultFaviconHrefs();
  for (const [link, href] of defaultFaviconHrefs) {
    if (link.isConnected) link.href = href;
  }
  currentStatus = null;
}
