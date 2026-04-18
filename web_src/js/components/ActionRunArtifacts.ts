import {html, htmlEscape} from '../utils/html.ts';
import {formatBytes} from '../utils.ts';
import type {ActionsArtifact} from '../modules/gitea-actions.ts';

export function buildArtifactTooltipHtml(artifact: ActionsArtifact, expiresAtLocale: string): string {
  const sizeText = formatBytes(artifact.size);
  if (artifact.expiresUnix <= 0) return htmlEscape(sizeText);

  const datetime = new Date(artifact.expiresUnix * 1000).toISOString();
  // split so the <relative-time> element can be interleaved, e.g. "Expires at %s" -> ["Expires at ", ""]
  const [prefix, suffix = ''] = expiresAtLocale.split('%s');
  const relativeTime = html`<relative-time datetime="${datetime}" threshold="P0Y" prefix="" weekday="" year="numeric" month="short" hour="numeric" minute="2-digit"></relative-time>`;
  const sizeSpan = html`<span class="artifact-size tw-border-l tw-border-current tw-ml-2 tw-pl-2">${sizeText}</span>`;
  return htmlEscape(prefix) + relativeTime + htmlEscape(suffix) + sizeSpan;
}
