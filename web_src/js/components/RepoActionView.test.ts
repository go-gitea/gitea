import {shouldHideLine, type LogLine} from './RepoActionView.vue';

test('shouldHideLine', () => {
  expect(([
    {index: 1, message: 'Starting build process', timestamp: 1000},
    {index: 2, message: '::add-matcher::/home/runner/go/pkg/mod/example.com/tool/matcher.json', timestamp: 1001},
    {index: 3, message: 'Running tests...', timestamp: 1002},
    {index: 4, message: '##[add-matcher]/opt/hostedtoolcache/go/1.25.7/x64/matchers.json', timestamp: 1003},
    {index: 5, message: 'Test suite started', timestamp: 1004},
    {index: 7, message: 'All tests passed', timestamp: 1006},
    {index: 8, message: '::remove-matcher owner=go::', timestamp: 1007},
    {index: 9, message: 'Build complete', timestamp: 1008},
  ] as Array<LogLine>).filter((line) => !shouldHideLine(line)).map((line) => line.message)).toMatchInlineSnapshot(`
    [
      "Starting build process",
      "Running tests...",
      "Test suite started",
      "All tests passed",
      "Build complete",
    ]
  `);
});
