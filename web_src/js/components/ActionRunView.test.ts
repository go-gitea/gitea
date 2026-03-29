import {createLogLineMessage, parseLogLineCommand} from './ActionRunView.ts';

test('LogLineMessage', () => {
  const cases = {
    'normal message': '<span class="log-msg">normal message</span>',
    '##[group] foo': '<span class="log-msg log-cmd-group"> foo</span>',
    '::group::foo': '<span class="log-msg log-cmd-group">foo</span>',
    '##[endgroup]': '<span class="log-msg log-cmd-endgroup"></span>',
    '::endgroup::': '<span class="log-msg log-cmd-endgroup"></span>',

    '##[error] foo': '<span class="log-msg log-cmd-error"><span class="log-msg-label">Error:</span><span> foo</span></span>',
    '##[warning] foo': '<span class="log-msg log-cmd-warning"><span class="log-msg-label">Warning:</span><span> foo</span></span>',
    '##[notice] foo': '<span class="log-msg log-cmd-notice"><span class="log-msg-label">Notice:</span><span> foo</span></span>',
    '##[debug] foo': '<span class="log-msg log-cmd-debug"><span class="log-msg-label">Debug:</span><span> foo</span></span>',
    '::error::foo': '<span class="log-msg log-cmd-error"><span class="log-msg-label">Error:</span><span> foo</span></span>',
    '::warning file=test.js,line=1::foo': '<span class="log-msg log-cmd-warning"><span class="log-msg-label">Warning:</span><span> foo</span></span>',
    '::notice::foo': '<span class="log-msg log-cmd-notice"><span class="log-msg-label">Notice:</span><span> foo</span></span>',
    '::debug::foo': '<span class="log-msg log-cmd-debug"><span class="log-msg-label">Debug:</span><span> foo</span></span>',
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
