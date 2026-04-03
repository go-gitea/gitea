import {createArtifactTooltipContent, formatArtifactSize, formatArtifactTimestamp} from './ActionRunArtifacts.ts';

test('createArtifactTooltipContent', () => {
  document.documentElement.lang = 'en-US';
  const locale = {
    artifactExpired: 'Expired',
    artifactExpiresAt: 'Expires at',
    artifactSize: 'Size',
    status: {
      unknown: 'Unknown',
    },
  };

  expect(createArtifactTooltipContent({
    name: 'artifact.zip',
    size: 1536,
    status: 'completed',
    expiresUnix: Date.UTC(2026, 2, 20, 12, 0, 0) / 1000,
  }, locale)).toContain('Expires at:');

  expect(createArtifactTooltipContent({
    name: 'artifact.zip',
    size: 0,
    status: 'expired',
    expiresUnix: 0,
  }, locale)).toBe('Expired | Size: 0 B');
});

test('formatArtifactTimestamp', () => {
  document.documentElement.lang = 'en-US';
  expect(formatArtifactTimestamp(0)).toBeNull();
  expect(formatArtifactTimestamp(Number.NaN)).toBeNull();
});

test('formatArtifactSize', () => {
  expect(formatArtifactSize(0)).toBe('0 B');
  expect(formatArtifactSize(1536)).toBe('1.5 KiB');
});
