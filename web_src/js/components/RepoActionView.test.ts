// Test for workflow command line filtering logic

// These are the constants and functions from RepoActionView.vue
const LogLinePrefixesHidden = ['::add-matcher::', '##[add-matcher]', '::workflow-command', '::remove-matcher'];

type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

function shouldHideLine(line: LogLine): boolean {
  for (const prefix of LogLinePrefixesHidden) {
    if (line.message.startsWith(prefix)) {
      return true;
    }
  }
  return false;
}

test('shouldHideLine filters workflow commands starting with ::add-matcher::', () => {
  const line: LogLine = {
    index: 1,
    timestamp: 1000,
    message: '::add-matcher::.github/problem-matcher.json',
  };
  expect(shouldHideLine(line)).toBe(true);
});

test('shouldHideLine filters workflow commands starting with ##[add-matcher]', () => {
  const line: LogLine = {
    index: 2,
    timestamp: 1001,
    message: '##[add-matcher].github/eslint.json',
  };
  expect(shouldHideLine(line)).toBe(true);
});

test('shouldHideLine filters workflow commands starting with ::workflow-command', () => {
  const line: LogLine = {
    index: 3,
    timestamp: 1002,
    message: '::workflow-command::some-command',
  };
  expect(shouldHideLine(line)).toBe(true);
});

test('shouldHideLine filters workflow commands starting with ::remove-matcher', () => {
  const line: LogLine = {
    index: 4,
    timestamp: 1003,
    message: '::remove-matcher::owner=eslint',
  };
  expect(shouldHideLine(line)).toBe(true);
});

test('shouldHideLine does not filter normal log lines', () => {
  const line: LogLine = {
    index: 5,
    timestamp: 1004,
    message: 'Normal log line without workflow commands',
  };
  expect(shouldHideLine(line)).toBe(false);
});

test('shouldHideLine does not filter lines with workflow commands not at the start', () => {
  const line: LogLine = {
    index: 6,
    timestamp: 1005,
    message: 'Some text before ::add-matcher:: in the middle',
  };
  expect(shouldHideLine(line)).toBe(false);
});

test('shouldHideLine handles group commands (should not hide them)', () => {
  const groupLine: LogLine = {
    index: 7,
    timestamp: 1006,
    message: '::group::Build Step',
  };
  expect(shouldHideLine(groupLine)).toBe(false);

  const endGroupLine: LogLine = {
    index: 8,
    timestamp: 1007,
    message: '::endgroup::',
  };
  expect(shouldHideLine(endGroupLine)).toBe(false);
});

test('shouldHideLine handles various log formats', () => {
  const testCases = [
    {message: '::add-matcher::', expected: true},
    {message: '##[add-matcher]', expected: true},
    {message: '::workflow-command', expected: true},
    {message: '::remove-matcher', expected: true},
    {message: 'Build started', expected: false},
    {message: 'Error: test failed', expected: false},
    {message: '  ::add-matcher::.github/test.json', expected: false}, // with leading space
  ];

  for (const [index, {message, expected}] of testCases.entries()) {
    const line: LogLine = {
      index: index + 10,
      timestamp: 2000 + index,
      message,
    };
    expect(shouldHideLine(line)).toBe(expected);
  }
});
