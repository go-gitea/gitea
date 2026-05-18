import {readFile} from 'node:fs/promises';
import {join} from 'node:path';

type CssVariables = Record<string, string>;

async function loadThemeVariables(fileName: string, baseVariables: CssVariables = {}): Promise<CssVariables> {
  const themePath = join(import.meta.dirname, '../../css/themes', fileName);
  const css = await readFile(themePath, 'utf8');
  const variables = {...baseVariables};
  for (const match of css.matchAll(/(--[\w-]+):\s*(#[\dA-Fa-f]{6});/g)) {
    variables[match[1]] = match[2];
  }
  return variables;
}

function relativeLuminance(hex: string): number {
  const rgb = [
    Number.parseInt(hex.slice(1, 3), 16),
    Number.parseInt(hex.slice(3, 5), 16),
    Number.parseInt(hex.slice(5, 7), 16),
  ].map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
}

function contrastRatio(foreground: string, background: string): number {
  const foregroundLuminance = relativeLuminance(foreground);
  const backgroundLuminance = relativeLuminance(background);
  return (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) /
    (Math.min(foregroundLuminance, backgroundLuminance) + 0.05);
}

function expectWcagAaaContrast(label: string, foreground: string, background: string): void {
  expect(contrastRatio(foreground, background), label).toBeGreaterThanOrEqual(7);
}

test('dark diff stat colors have WCAG AAA contrast', async () => {
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
    expectWcagAaaContrast(label, variables[colorVariable], darkVariables['--color-body']);
  }
});

test('dark syntax name colors have WCAG AAA contrast on diff rows', async () => {
  const variables = await loadThemeVariables('theme-gitea-dark.css');
  const backgrounds = ['--color-diff-added-row-bg', '--color-diff-removed-row-bg'] as const;
  const foregrounds = ['--color-syntax-name', '--color-syntax-type'] as const;

  for (const foreground of foregrounds) {
    for (const background of backgrounds) {
      expectWcagAaaContrast(`${foreground} on ${background}`, variables[foreground], variables[background]);
    }
  }
});
