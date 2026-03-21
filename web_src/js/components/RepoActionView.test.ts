import {createLogLineMessage, parseLogLineCommand} from './RepoActionView.vue';

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
