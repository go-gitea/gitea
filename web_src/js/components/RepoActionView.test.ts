import {createArtifactTooltip, createLogLineMessage, formatArtifactTimestamp, parseLogLineCommand} from './RepoActionView.vue';

test('LogLineMessage', () => {
  const cases = {
    'normal message': '<span class="log-msg">normal message</span>',
    '##[group] foo': '<span class="log-msg log-cmd-group"> foo</span>',
    '::group::foo': '<span class="log-msg log-cmd-group">foo</span>',
    '##[endgroup]': '<span class="log-msg log-cmd-endgroup"></span>',
    '::endgroup::': '<span class="log-msg log-cmd-endgroup"></span>',

    // parser shouldn't do any trim, keep origin output as-is
    '##[error] foo': '<span class="log-msg log-cmd-error"> foo</span>',
    '[command] foo': '<span class="log-msg log-cmd-command"> foo</span>',

    // hidden is special, it is actually skipped before creating
    '##[add-matcher]foo': '<span class="log-msg log-cmd-hidden">foo</span>',
    '::add-matcher::foo': '<span class="log-msg log-cmd-hidden">foo</span>',
    '::remove-matcher foo::': '<span class="log-msg log-cmd-hidden"> foo::</span>', // not correctly parsed, but we don't need it
  };
  for (const [input, html] of Object.entries(cases)) {
    const line = {index: 0, timestamp: 0, message: input};
    const cmd = parseLogLineCommand(line);
    const el = createLogLineMessage(line, cmd);
    expect(el.outerHTML).toBe(html);
  }
});

test('createArtifactTooltip', () => {
  document.documentElement.lang = 'en-US';
  const locale = {
    artifactExpires: 'Expires',
    artifactStatus: 'Status',
    artifactExpired: 'Expired',
    status: {
      unknown: 'Unknown',
    },
  };

  expect(createArtifactTooltip({
    name: 'artifact.zip',
    size: 1536,
    status: 'completed',
    expiresAt: '2026-03-20T12:00:00Z',
  }, locale)).toContain('Expires:');

  expect(createArtifactTooltip({
    name: 'artifact.zip',
    size: 0,
    status: 'expired',
  }, locale)).toBe('Expires: Unknown\nStatus: Expired');
});

test('formatArtifactTimestamp', () => {
  document.documentElement.lang = 'en-US';
  expect(formatArtifactTimestamp('0001-01-01T00:00:00Z', 'Unknown')).toBe('Unknown');
  expect(formatArtifactTimestamp(undefined, 'Unknown')).toBe('Unknown');
});
