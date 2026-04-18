import {buildArtifactTooltipHtml} from './ActionRunArtifacts.ts';

test('buildArtifactTooltipHtml for active artifact', () => {
  const result = buildArtifactTooltipHtml({
    name: 'artifact.zip',
    size: 1024 * 1024,
    status: 'completed',
    expiresUnix: Date.UTC(2026, 2, 20, 12, 0, 0) / 1000,
  }, 'Expires at %s');

  expect(result).toContain('<relative-time datetime="2026-03-20T12:00:00.000Z"');
  expect(result).toContain('threshold="P0Y"');
  expect(result).toContain('month="short"');
  expect(result).toContain('hour="numeric"');
  expect(result).toContain('minute="2-digit"');
  expect(result).toContain('Expires at');
  expect(result).toContain('1.0 MiB');
  expect(result).toContain('class="artifact-size');
});

test('buildArtifactTooltipHtml with no expiry', () => {
  const result = buildArtifactTooltipHtml({
    name: 'artifact.zip',
    size: 512,
    status: 'completed',
    expiresUnix: 0,
  }, 'Expires at %s');

  expect(result).not.toContain('<relative-time');
  expect(result).toBe('512 B');
});
