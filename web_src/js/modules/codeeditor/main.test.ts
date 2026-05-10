import {buildLanguageDescriptions, importCodemirror} from './main.ts';

test('matchFilename — language detection covers extended rules', async () => {
  const cm = await importCodemirror();
  const list = buildLanguageDescriptions(cm);
  const match = (filename: string) =>
    cm.language.LanguageDescription.matchFilename(list, filename)?.name;

  expect(match('.bashrc')).toBe('Shell');
  expect(match('.zshrc')).toBe('Shell');
  expect(match('.envrc')).toBe('Shell');
  expect(match('foo.zsh')).toBe('Shell');
  expect(match('foo.bats')).toBe('Shell');
  expect(match('PKGBUILD')).toBe('Shell');

  expect(match('.gitconfig')).toBe('Properties files');
  expect(match('.editorconfig')).toBe('Properties files');
  expect(match('.npmrc')).toBe('Properties files');
  expect(match('foo.cfg')).toBe('Properties files');
  expect(match('foo.conf')).toBe('Properties files');
  expect(match('nginx.conf')).toBe('Nginx');

  expect(match('Cargo.lock')).toBe('TOML');
  expect(match('Pipfile')).toBe('TOML');
  expect(match('poetry.lock')).toBe('TOML');

  expect(match('Containerfile')).toBe('Dockerfile');
  expect(match('Containerfile.test')).toBe('Dockerfile');
  expect(match('Dockerfile')).toBe('Dockerfile');
  expect(match('Dockerfile.dev')).toBe('Dockerfile');

  expect(match('Brewfile')).toBe('Ruby');
  expect(match('Vagrantfile')).toBe('Ruby');
  expect(match('Gemfile')).toBe('Ruby');
  expect(match('foo.gemspec')).toBe('Ruby');
  expect(match('foo.rake')).toBe('Ruby');
  expect(match('foo.ru')).toBe('Ruby');

  expect(match('foo.psgi')).toBe('Perl');
  expect(match('foo.pyi')).toBe('Python');
  expect(match('Snakefile')).toBe('Python');

  expect(match('foo.webmanifest')).toBe('JSON');
  expect(match('foo.geojson')).toBe('JSON');
  expect(match('composer.lock')).toBe('JSON');
  expect(match('bun.lock')).toBe('JSON');

  expect(match('foo.tcc')).toBe('C++');
  expect(match('foo.tpp')).toBe('C++');
  expect(match('foo.cppm')).toBe('C++');
  expect(match('foo.ixx')).toBe('C++');

  expect(match('foo.xhtml')).toBe('HTML');
  expect(match('foo.jsh')).toBe('Java');
  expect(match('.Rprofile')).toBe('R');

  expect(match('Makefile')).toBe('Makefile');
  expect(match('Makefile.am')).toBe('Makefile');
  expect(match('BSDmakefile')).toBe('Makefile');
  expect(match('GNUmakefile')).toBe('Makefile');
  expect(match('foo.mk')).toBe('Makefile');

  expect(match('.env')).toBe('Dotenv');
  expect(match('.env.local')).toBe('Dotenv');

  expect(match('foo.md')).toBe('Markdown');
  expect(match('foo.mdown')).toBe('Markdown');
  expect(match('foo.json5')).toBe('JSON5');
  expect(match('tsconfig.json')).toBe('JSON');

  // Smoke tests for languages that already worked, to guard against regressions.
  expect(match('foo.go')).toBe('Go');
  expect(match('foo.rs')).toBe('Rust');
  expect(match('foo.ts')).toBe('TypeScript');
  expect(match('foo.tsx')).toBe('TSX');
  expect(match('foo.py')).toBe('Python');
  expect(match('foo.html')).toBe('HTML');
  expect(match('foo.css')).toBe('CSS');
  expect(match('foo.lua')).toBe('Lua');

  // Extension co-claimed by XML and another language; the other should win on order.
  expect(match('app.csproj')).toBe('XML');
  expect(match('foo.jsproj')).toBe('XML');

  // .spec must route to RPM Spec, not Python/Ruby, despite Linguist's primary.
  expect(match('foo.spec')).toBe('RPM Spec');

  // Genuinely ambiguous extensions left unhighlighted on purpose.
  expect(match('foo.cgi')).toBeUndefined();
  expect(match('foo.fcgi')).toBeUndefined();
  expect(match('foo.inc')).toBeUndefined();
  expect(match('foo.fish')).toBeUndefined();
});
