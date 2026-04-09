import {createElementFromAttrs} from '../utils/dom.ts';
import {formatBytes} from '../utils.ts';
import type {ActionsArtifact} from '../modules/gitea-actions.ts';

export function createArtifactTooltipElement(artifact: ActionsArtifact, expiresAtLocale: string): HTMLElement {
  const sizeText = formatBytes(artifact.size);

  if (artifact.expiresUnix <= 0) {
    return createElementFromAttrs('span', null, sizeText);
  }

  const datetime = new Date(artifact.expiresUnix * 1000).toISOString();
  const parts = expiresAtLocale.split('%s');
  const relativeTime = createElementFromAttrs('relative-time', {
    datetime, threshold: 'P0Y', prefix: '', weekday: '',
    year: 'numeric', month: 'short', hour: 'numeric', minute: 'numeric',
  });
  return createElementFromAttrs('span', null, parts[0] ?? '', relativeTime, `${parts[1] ?? ''} | ${sizeText}`);
}
