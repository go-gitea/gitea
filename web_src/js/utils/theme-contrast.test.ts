import {readFile} from 'node:fs/promises';
import {join} from 'node:path';
import {colord, extend} from 'colord';
import a11yPlugin from 'colord/plugins/a11y';

extend([a11yPlugin]); // adds the WCAG-correct colord().contrast()

type CssVariables = Record<string, string>;

// Diff stat counters are short glanceable labels; issue #37448 asks for a contrast ratio of >= 5.
const diffStatMinContrast = 5;
// Function/type names are code read continuously, so keep the pre-1.26 AAA level.
const syntaxNameMinContrast = 7;
// A distinct type color only needs to stay comfortably readable (WCAG AA).
const syntaxTypeMinContrast = 4.5;

async function loadThemeVariables(fileName: string, baseVariables: CssVariables = {}): Promise<CssVariables> {
  const themePath = join(import.meta.dirname, '../../css/themes', fileName);
  const css = await readFile(themePath, 'utf8');
  const variables = {...baseVariables};
  for (const match of css.matchAll(/(--[\w-]+):\s*(#[\dA-Fa-f]{6});/g)) {
    variables[match[1]] = match[2];
  }
  return variables;
}

function expectMinContrast(label: string, foreground: string, background: string, min: number): void {
  expect(colord(foreground).contrast(background), label).toBeGreaterThanOrEqual(min);
}

test('dark diff stat colors meet the minimum contrast floor', async () => {
  const darkVariables = await loadThemeVariables('theme-gitea-dark.css');
  const colorblindVariables = await loadThemeVariables('theme-gitea-dark-protanopia-deuteranopia.css', darkVariables);
  const tritanopiaVariables = await loadThemeVariables('theme-gitea-dark-tritanopia.css', darkVariables);
  const themes: Array<[string, CssVariables, string]> = [
    ['dark added', darkVariables, '--color-diff-added-fg'],
    ['dark removed', darkVariables, '--color-diff-removed-fg'],
    ['dark protanopia/deuteranopia added', colorblindVariables, '--color-diff-added-fg'],
    ['dark protanopia/deuteranopia removed', colorblindVariables, '--color-diff-removed-fg'],
    ['dark tritanopia added', tritanopiaVariables, '--color-diff-added-fg'],
  ];

  for (const [label, variables, colorVariable] of themes) {
    expectMinContrast(label, variables[colorVariable], darkVariables['--color-body'], diffStatMinContrast);
  }
});

test('dark syntax name stays AAA on diff rows and distinct from type', async () => {
  const variables = await loadThemeVariables('theme-gitea-dark.css');
  const backgrounds = ['--color-diff-added-row-bg', '--color-diff-removed-row-bg'] as const;

  expect(variables['--color-syntax-name']).not.toBe(variables['--color-syntax-type']);
  for (const background of backgrounds) {
    expectMinContrast(`name on ${background}`, variables['--color-syntax-name'], variables[background], syntaxNameMinContrast);
    expectMinContrast(`type on ${background}`, variables['--color-syntax-type'], variables[background], syntaxTypeMinContrast);
  }
});
