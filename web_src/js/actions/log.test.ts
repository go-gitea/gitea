import {shouldHideLine, type LogLine} from './log.ts';

function filterLogLines(logLines: Array<LogLine>): Array<LogLine> {
  return logLines.filter((line) => !shouldHideLine(line));
}

test('filters workflow command', () => {
  expect(filterLogLines([
    {index: 1, message: 'Starting build process', timestamp: 1000},
    {index: 2, message: '::add-matcher::.github/problem-matcher.json', timestamp: 1001},
    {index: 3, message: 'Running tests...', timestamp: 1002},
    {index: 4, message: '##[add-matcher].github/eslint.json', timestamp: 1003},
    {index: 5, message: 'Test suite started', timestamp: 1004},
    {index: 6, message: '::workflow-command::echo some-output', timestamp: 1005},
    {index: 7, message: 'All tests passed', timestamp: 1006},
    {index: 8, message: '::remove-matcher::owner=eslint', timestamp: 1007},
    {index: 9, message: 'Build complete', timestamp: 1008},
  ]).map((line) => line.message)).toMatchInlineSnapshot(`
    [
      "Starting build process",
      "Running tests...",
      "Test suite started",
      "::workflow-command::echo some-output",
      "All tests passed",
      "Build complete",
    ]
  `);

  expect(filterLogLines([
    {index: 1, message: 'Normal log line', timestamp: 1000},
    {index: 2, message: '::group::Installation', timestamp: 1001},
    {index: 3, message: 'Installing dependencies', timestamp: 1002},
    {index: 4, message: '::add-matcher::.github/npm.json', timestamp: 1003},
    {index: 5, message: '::endgroup::', timestamp: 1004},
    {index: 6, message: 'Done', timestamp: 1005},
  ]).map((line) => line.message)).toMatchInlineSnapshot(`
    [
      "Normal log line",
      "::group::Installation",
      "Installing dependencies",
      "::endgroup::",
      "Done",
    ]
  `);
});
