import {readFile} from 'node:fs/promises';
import * as path from 'node:path';
import {globCompile} from './glob.ts';

async function loadGlobTestData(): Promise<{caseNames: string[], caseDataMap: Record<string, string>}> {
  const fileContent = await readFile(path.join(import.meta.dirname, 'glob.test.txt'), 'utf8');
  const fileLines = fileContent.split('\n');
  const caseDataMap: Record<string, string> = {};
  const caseNameMap: Record<string, boolean> = {};
  for (let line of fileLines) {
    line = line.trim();
    if (!line || line.startsWith('#')) continue;
    const parts = line.split('=', 2);
    if (parts.length !== 2) throw new Error(`Invalid test case line: ${line}`);

    const key = parts[0].trim();
    let value = parts[1].trim();
    value = value.substring(1, value.length - 1); // remove quotes
    value = value.replace(/\\\//g, '/').replace(/\\\\/g, '\\');
    caseDataMap[key] = value;
    if (key.startsWith('pattern_')) caseNameMap[key.substring('pattern_'.length)] = true;
  }
  return {caseNames: Object.keys(caseNameMap), caseDataMap};
}

function loadGlobGolangCases() {
  // https://github.com/gobwas/glob/blob/master/glob_test.go
  function glob(matched: boolean, pattern: string, input: string, separators: string = '') {
    return {matched, pattern, input, separators};
  }
  return [
    glob(true, '* ?at * eyes', 'my cat has very bright eyes'),

    glob(true, '', ''),
    glob(false, '', 'b'),

    glob(true, '*ä', 'åä'),
    glob(true, 'abc', 'abc'),
    glob(true, 'a*c', 'abc'),
    glob(true, 'a*c', 'a12345c'),
    glob(true, 'a?c', 'a1c'),
    glob(true, 'a.b', 'a.b', '.'),
    glob(true, 'a.*', 'a.b', '.'),
    glob(true, 'a.**', 'a.b.c', '.'),
    glob(true, 'a.?.c', 'a.b.c', '.'),
    glob(true, 'a.?.?', 'a.b.c', '.'),
    glob(true, '?at', 'cat'),
    glob(true, '?at', 'fat'),
    glob(true, '*', 'abc'),
    glob(true, `\\*`, '*'),
    glob(true, '**', 'a.b.c', '.'),

    glob(false, '?at', 'at'),
    glob(false, '?at', 'fat', 'f'),
    glob(false, 'a.*', 'a.b.c', '.'),
    glob(false, 'a.?.c', 'a.bb.c', '.'),
    glob(false, '*', 'a.b.c', '.'),

    glob(true, '*test', 'this is a test'),
    glob(true, 'this*', 'this is a test'),
    glob(true, '*is *', 'this is a test'),
    glob(true, '*is*a*', 'this is a test'),
    glob(true, '**test**', 'this is a test'),
    glob(true, '**is**a***test*', 'this is a test'),

    glob(false, '*is', 'this is a test'),
    glob(false, '*no*', 'this is a test'),
    glob(true, '[!a]*', 'this is a test3'),

    glob(true, '*abc', 'abcabc'),
    glob(true, '**abc', 'abcabc'),
    glob(true, '???', 'abc'),
    glob(true, '?*?', 'abc'),
    glob(true, '?*?', 'ac'),
    glob(false, 'sta', 'stagnation'),
    glob(true, 'sta*', 'stagnation'),
    glob(false, 'sta?', 'stagnation'),
    glob(false, 'sta?n', 'stagnation'),

    glob(true, '{abc,def}ghi', 'defghi'),
    glob(true, '{abc,abcd}a', 'abcda'),
    glob(true, '{a,ab}{bc,f}', 'abc'),
    glob(true, '{*,**}{a,b}', 'ab'),
    glob(false, '{*,**}{a,b}', 'ac'),

    glob(true, '/{rate,[a-z][a-z][a-z]}*', '/rate'),
    glob(true, '/{rate,[0-9][0-9][0-9]}*', '/rate'),
    glob(true, '/{rate,[a-z][a-z][a-z]}*', '/usd'),

    glob(true, '{*.google.*,*.yandex.*}', 'www.google.com', '.'),
    glob(true, '{*.google.*,*.yandex.*}', 'www.yandex.com', '.'),
    glob(false, '{*.google.*,*.yandex.*}', 'yandex.com', '.'),
    glob(false, '{*.google.*,*.yandex.*}', 'google.com', '.'),

    glob(true, '{*.google.*,yandex.*}', 'www.google.com', '.'),
    glob(true, '{*.google.*,yandex.*}', 'yandex.com', '.'),
    glob(false, '{*.google.*,yandex.*}', 'www.yandex.com', '.'),
    glob(false, '{*.google.*,yandex.*}', 'google.com', '.'),

    glob(true, '*//{,*.}example.com', 'https://www.example.com'),
    glob(true, '*//{,*.}example.com', 'http://example.com'),
    glob(false, '*//{,*.}example.com', 'http://example.com.net'),
  ];
}

test('GlobCompiler', async () => {
  const {caseNames, caseDataMap} = await loadGlobTestData();
  expect(caseNames.length).toBe(10); // should have 10 test cases
  for (const caseName of caseNames) {
    const pattern = caseDataMap[`pattern_${caseName}`];
    const regexp = caseDataMap[`regexp_${caseName}`];
    expect(globCompile(pattern).regexpPattern).toBe(regexp);
  }

  const golangCases = loadGlobGolangCases();
  expect(golangCases.length).toBe(60);
  for (const c of golangCases) {
    const compiled = globCompile(c.pattern, c.separators);
    const msg = `pattern: ${c.pattern}, input: ${c.input}, separators: ${c.separators || '(none)'}, compiled: ${compiled.regexpPattern}`;
    // eslint-disable-next-line @vitest/valid-expect -- Unlike Jest, Vitest supports a message as the second argument
    expect(compiled.regexp.test(c.input), msg).toBe(c.matched);
  }

  // then our cases
  expect(globCompile('*/**/x').regexpPattern).toBe('^.*/.*/x$');
  expect(globCompile('*/**/x', '/').regexpPattern).toBe('^[^/]*/.*/x$');
  expect(globCompile('[a-b][^-\\]]', '/').regexpPattern).toBe('^[a-b][^-\\]]$');
  expect(globCompile('.+^$()|', '/').regexpPattern).toBe('^\\.\\+\\^\\$\\(\\)\\|$');
});
