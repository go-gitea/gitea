import {createArtifactTooltipElement} from './ActionRunArtifacts.ts';

test('createArtifactTooltipElement for active artifact', () => {
  const el = createArtifactTooltipElement({
    name: 'artifact.zip',
    size: 1024 * 1024,
    status: 'completed',
    expiresUnix: Date.UTC(2026, 2, 20, 12, 0, 0) / 1000,
  }, 'Expires at %s');

  const rt = el.querySelector('relative-time')!;
  expect(rt).not.toBeNull();
  expect(rt.getAttribute('datetime')).toBe('2026-03-20T12:00:00.000Z');
  expect(rt.getAttribute('threshold')).toBe('P0Y');
  expect(rt.getAttribute('month')).toBe('short');
  expect(rt.getAttribute('hour')).toBe('numeric');
  expect(rt.getAttribute('minute')).toBe('numeric');
  expect(el.textContent).toContain('Expires at');
  expect(el.textContent).toContain('1.0 MiB');
});

test('createArtifactTooltipElement with no expiry', () => {
  const el = createArtifactTooltipElement({
    name: 'artifact.zip',
    size: 512,
    status: 'completed',
    expiresUnix: 0,
  }, 'Expires at %s');

  expect(el.querySelector('relative-time')).toBeNull();
  expect(el.textContent).toBe('512 B');
});
