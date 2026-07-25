import {createLogLineMessage, getAnsiRender, parseLogLineCommand} from './ActionRunView.ts';

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
    '##[command] foo': '<span class="log-msg log-cmd-command"> foo</span>',
    '[command] foo': '<span class="log-msg log-cmd-command"> foo</span>',

    // hidden is special, it is actually skipped before creating
    '##[add-matcher]foo': '<span class="log-msg log-cmd-hidden">foo</span>',
    '::add-matcher::foo': '<span class="log-msg log-cmd-hidden">foo</span>',
    '::remove-matcher foo::': '<span class="log-msg log-cmd-hidden"> foo::</span>', // not correctly parsed, but we don't need it
  };
  const ansiRender = getAnsiRender();
  for (const [input, html] of Object.entries(cases)) {
    const line = {index: 0, timestamp: 0, message: input};
    const cmd = parseLogLineCommand(line);
    const el = createLogLineMessage(ansiRender, line, cmd);
    expect(el.outerHTML).toBe(html);
  }
});

test('getAnsiRender', () => {
  // keep rendering new lines, use the same render
  const r1 = getAnsiRender(0, {index: 0, timestamp: 0, message: ''});
  const r1a = getAnsiRender(0, {index: 1, timestamp: 0, message: ''});
  expect(r1).toBe(r1a);
  const r1b = getAnsiRender(0, {index: 2, timestamp: 0, message: ''});
  expect(r1).toBe(r1b);

  // the line index doens't match, use a new render
  const r2 = getAnsiRender(0, {index: 1, timestamp: 0, message: ''});
  expect(r1).not.toBe(r2);
});
