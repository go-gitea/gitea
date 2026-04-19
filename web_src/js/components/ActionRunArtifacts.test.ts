import {buildArtifactTooltipHtml} from './ActionRunArtifacts.ts';
import {normalizeTestHtml} from '../utils/testhelper.ts';

describe('buildArtifactTooltipHtml', () => {
  test('active artifact', () => {
    const result = buildArtifactTooltipHtml({
      name: 'artifact.zip',
      size: 1024 * 1024,
      status: 'completed',
      expiresUnix: Date.UTC(2026, 2, 20, 12, 0, 0) / 1000,
    }, 'Expires at %s (extra)');

    expect(normalizeTestHtml(result)).toBe(normalizeTestHtml(`<span class="flex-text-inline">
<span>Expires at </span>
  <relative-time datetime="2026-03-20T12:00:00.000Z" threshold="P0Y" prefix="" weekday="" year="numeric" month="short" hour="numeric" minute="2-digit">
    2026-03-20T12:00:00.000Z
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
