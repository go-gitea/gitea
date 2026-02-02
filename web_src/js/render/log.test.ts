import {shouldHideLine, type LogLine} from './log.ts';

function filterLogLines(logLines: LogLine[]): LogLine[] {
  return logLines.filter((line) => !shouldHideLine(line));
}

test('filters workflow command lines from log output', () => {
  const inputLogLines: LogLine[] = [
    {index: 1, timestamp: 1000, message: 'Starting build process'},
    {index: 2, timestamp: 1001, message: '::add-matcher::.github/problem-matcher.json'},
    {index: 3, timestamp: 1002, message: 'Running tests...'},
    {index: 4, timestamp: 1003, message: '##[add-matcher].github/eslint.json'},
    {index: 5, timestamp: 1004, message: 'Test suite started'},
    {index: 6, timestamp: 1005, message: '::workflow-command::echo some-output'},
    {index: 7, timestamp: 1006, message: 'All tests passed'},
    {index: 8, timestamp: 1007, message: '::remove-matcher::owner=eslint'},
    {index: 9, timestamp: 1008, message: 'Build complete'},
  ];

  const expectedVisibleMessages = [
    'Starting build process',
    'Running tests...',
    'Test suite started',
    '::workflow-command::echo some-output',
    'All tests passed',
    'Build complete',
  ];

  const visibleLines = filterLogLines(inputLogLines);

  expect(visibleLines.length).toBe(6);

  const visibleMessages = visibleLines.map((line) => line.message);
  expect(visibleMessages).toEqual(expectedVisibleMessages);

  expect(visibleMessages).not.toContain('::add-matcher::.github/problem-matcher.json');
  expect(visibleMessages).not.toContain('##[add-matcher].github/eslint.json');
  expect(visibleMessages).toContain('::workflow-command::echo some-output');
  expect(visibleMessages).not.toContain('::remove-matcher::owner=eslint');
});

test('preserves non-workflow command lines including group commands', () => {
  const inputLogLines: LogLine[] = [
    {index: 1, timestamp: 1000, message: 'Normal log line'},
    {index: 2, timestamp: 1001, message: '::group::Installation'},
    {index: 3, timestamp: 1002, message: 'Installing dependencies'},
    {index: 4, timestamp: 1003, message: '::add-matcher::.github/npm.json'},
    {index: 5, timestamp: 1004, message: '::endgroup::'},
    {index: 6, timestamp: 1005, message: 'Done'},
  ];

  const visibleLines = filterLogLines(inputLogLines);

  expect(visibleLines.length).toBe(5);

  const visibleMessages = visibleLines.map((line) => line.message);
  expect(visibleMessages).toContain('::group::Installation');
  expect(visibleMessages).toContain('::endgroup::');
  expect(visibleMessages).not.toContain('::add-matcher::.github/npm.json');
});
