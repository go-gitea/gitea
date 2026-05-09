import {html} from '../utils/html.ts';
import {formatBytes} from '../utils.ts';
import type {ActionsArtifact} from '../modules/gitea-actions.ts';

export function buildArtifactTooltipHtml(artifact: ActionsArtifact, expiresAtLocale: string): string {
  const sizeText = formatBytes(artifact.size);
  if (artifact.expiresUnix <= 0) {
    return html`<span class="flex-text-inline">${sizeText}</span>`; // use the same layout as below
  }
  const datetimeLocal = new Date(artifact.expiresUnix * 1000).toLocaleString();
  // split so the <relative-time> element can be interleaved, e.g. "Expires at %s" -> ["Expires at ", ""]
  const [prefix, suffix = ''] = expiresAtLocale.split('%s');
  return html`
    <span class="flex-text-inline">
      <span>${prefix}</span>
      <relative-time datetime="${artifact.expiresUnix}" threshold="P0Y" prefix="" weekday="" year="numeric" month="short" hour="numeric" minute="2-digit">
        ${datetimeLocal}
      </relative-time>
      <span>${suffix}</span>
      <span class="inline-divider">,</span>
      <span>${sizeText}</span>
    </span>
  `;
}
