import {buildArtifactTooltipHtml} from './ActionRunArtifacts.ts';
import {normalizeTestHtml} from '../utils/testhelper.ts';

describe('buildArtifactTooltipHtml', () => {
  test('active artifact', () => {
    const expiresUnix = Date.UTC(2026, 2, 20, 12, 0, 0) / 1000;
    const expiresLocal = new Date(expiresUnix * 1000).toLocaleString();
    const result = buildArtifactTooltipHtml({
      name: 'artifact.zip',
      size: 1024 * 1024,
      status: 'completed',
      expiresUnix,
    }, 'Expires at %s (extra)');

    expect(normalizeTestHtml(result)).toBe(normalizeTestHtml(`<span class="flex-text-inline">
<span>Expires at </span>
  <relative-time datetime="${expiresUnix}" threshold="P0Y" prefix="" weekday="" year="numeric" month="short" hour="numeric" minute="2-digit">
    ${expiresLocal}
  </relative-time>
  <span> (extra)</span>
  <span class="inline-divider">,</span>
  <span>1.0 MiB</span>
</span>
`));
  });

  test('no expiry', () => {
    const result = buildArtifactTooltipHtml({
      name: 'artifact.zip',
      size: 512,
      status: 'completed',
      expiresUnix: 0,
    }, 'Expires at %s');
    expect(normalizeTestHtml(result)).toBe(`<span class="flex-text-inline">512 B</span>`);
  });
});
