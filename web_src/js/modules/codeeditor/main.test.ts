import {buildLanguageDescriptions, importCodemirror} from './main.ts';

test('matchFilename — language detection covers extended rules', async () => {
  const cm = await importCodemirror();
  const list = buildLanguageDescriptions(cm);
  const match = (filename: string) =>
    cm.language.LanguageDescription.matchFilename(list, filename)?.name;

  // Linguist-supplied filenames + extensions
  expect(match('.bashrc')).toBe('Shell');
  expect(match('PKGBUILD')).toBe('Shell');
  expect(match('foo.zsh')).toBe('Shell');
  expect(match('Cargo.lock')).toBe('TOML');
  expect(match('Gemfile')).toBe('Ruby');
  expect(match('foo.gemspec')).toBe('Ruby');
  expect(match('foo.psgi')).toBe('Perl');
  expect(match('foo.pyi')).toBe('Python');
  expect(match('foo.webmanifest')).toBe('JSON');
  expect(match('foo.tcc')).toBe('C++');

  // Script-side extras (extraFilenames / extraExtensions)
  expect(match('.editorconfig')).toBe('Properties files');
  expect(match('foo.conf')).toBe('Properties files');
  expect(match('Snakefile')).toBe('Python');

  // Custom Gitea entries override language-data
  expect(match('Containerfile.test')).toBe('Dockerfile');
  expect(match('Dockerfile.dev')).toBe('Dockerfile');
  expect(match('Makefile.am')).toBe('Makefile');
  expect(match('foo.mk')).toBe('Makefile');
  expect(match('.env.local')).toBe('Dotenv');
  expect(match('foo.json5')).toBe('JSON5');
  expect(match('foo.mdown')).toBe('Markdown');

  // Filename regex wins over extension match
  expect(match('nginx.conf')).toBe('Nginx');

  // .spec routes to RPM Spec via excludeExt redirect
  expect(match('foo.spec')).toBe('RPM Spec');

  // CM original ownership preserved against Linguist's broader claims (.sql is SQL,
  // not PLSQL, even though Linguist's PLSQL extension list includes it).
  expect(match('foo.sql')).toBe('SQL');
  expect(match('foo.h')).toBe('C');
  expect(match('foo.mm')).toBe('Objective-C++');

  // Globally ambiguous extensions fall through to plain text
  expect(match('foo.cgi')).toBeUndefined();
  expect(match('foo.inc')).toBeUndefined();

  // Smoke: existing language-data entries still resolve
  expect(match('foo.go')).toBe('Go');
  expect(match('foo.tsx')).toBe('TSX');
});
