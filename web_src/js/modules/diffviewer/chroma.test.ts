import {ChromaLexer, loadChromaLexer, translatePattern} from './chroma.ts';

const lexerFiles = import.meta.glob<ConstructorParameters<typeof ChromaLexer>[0]>(['../../../../assets/chroma-lexers/*.json', '!**/index.json'], {import: 'default', eager: true});

test('all lexers compile', () => {
  let rules = 0;
  const failed: string[] = [];
  for (const file of Object.values(lexerFiles)) {
    const lexer = new ChromaLexer(file, () => null);
    for (const [state, stateRules] of lexer.rules) {
      for (const [idx, rule] of stateRules.entries()) {
        rules++;
        if (!rule.re) failed.push(`${lexer.name} ${state}.${idx}`);
      }
    }
  }
  // a few rules use regexp2 features JavaScript doesn't have, they never match
  expect(failed.length / rules, failed.join('\n')).toBeLessThan(0.002);
});

test('tokenize', async () => {
  const samples: Array<[string, string]> = [
    ['main.zig', 'const std = @import("std");\npub fn main() void {\n    // hello\n    const x: u32 = 0x1F;\n}\n'],
    ['main.tf', 'resource "aws_instance" "web" {\n  ami = "ami-123" # comment\n  count = 2\n}\n'],
    ['schema.graphql', 'type Query {\n  user(id: ID!): User\n}\n'],
    ['run.bat', '@echo off\nset NAME=world\necho Hello %NAME%\n'],
    ['Token.sol', 'pragma solidity ^0.8.0;\ncontract Token { uint256 public total = 1e18; }\n'],
  ];
  for (const [filename, code] of samples) {
    const lexer = (await loadChromaLexer(filename, ''))!;
    expect(lexer, filename).not.toBeNull();
    const tokens = lexer.tokenize(code);
    expect(tokens.map(([text]) => text).join(''), filename).toEqual(code);
    expect(tokens.some(([, classes]) => classes.startsWith('k')), filename).toBe(true);
  }
  expect((await loadChromaLexer('unknown.xyz123', ''))).toBeNull();
  expect((await loadChromaLexer('script', 'zig'))!.name).toEqual('Zig');
});

test('translatePattern', () => {
  expect(translatePattern('\\A(?>foo)++\\Z', true)).toEqual('^(?:foo)+$');
  expect(translatePattern('(?x) a # comment\n b', true)).toEqual('ab');
  expect(translatePattern('[]a]\\Q.*\\E', true)).toEqual('[\\]a]\\.\\*');
  expect(translatePattern('\\p{L}[\\p{N}_]', false)).toEqual('[a-zA-Z\\u00C0-\\uFFFF][0-9_]');
});
