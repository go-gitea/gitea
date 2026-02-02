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

  expect(filterLogLines(inputLogLines)).toMatchInlineSnapshot(`
    [
      {
        "index": 1,
        "message": "Starting build process",
        "timestamp": 1000,
      },
      {
        "index": 3,
        "message": "Running tests...",
        "timestamp": 1002,
      },
      {
        "index": 5,
        "message": "Test suite started",
        "timestamp": 1004,
      },
      {
        "index": 6,
        "message": "::workflow-command::echo some-output",
        "timestamp": 1005,
      },
      {
        "index": 7,
        "message": "All tests passed",
        "timestamp": 1006,
      },
      {
        "index": 9,
        "message": "Build complete",
        "timestamp": 1008,
      },
    ]
  `);
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

  expect(filterLogLines(inputLogLines)).toMatchInlineSnapshot(`
    [
      {
        "index": 1,
        "message": "Normal log line",
        "timestamp": 1000,
      },
      {
        "index": 2,
        "message": "::group::Installation",
        "timestamp": 1001,
      },
      {
        "index": 3,
        "message": "Installing dependencies",
        "timestamp": 1002,
      },
      {
        "index": 5,
        "message": "::endgroup::",
        "timestamp": 1004,
      },
      {
        "index": 6,
        "message": "Done",
        "timestamp": 1005,
      },
    ]
  `);
});
