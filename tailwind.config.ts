import {readFileSync} from 'node:fs';
import {parse} from 'postcss';
import type {Config} from 'tailwindcss';

function extractRootVars(css: string) {
  const root = parse(css);
  const vars = new Set<string>();
  root.walkRules((rule) => {
    if (rule.selector !== ':root') return;
    rule.each((node) => {
      if (node.type === 'decl' && node.value && node.prop.startsWith('--')) {
        vars.add(node.prop.substring(2));
      }
    });
  });
  return Array.from(vars);
}

const vars = extractRootVars([
  readFileSync(new URL('web_src/css/themes/theme-gitea-light.css', import.meta.url), 'utf8'),
  readFileSync(new URL('web_src/css/themes/theme-gitea-dark.css', import.meta.url), 'utf8'),
].join('\n'));

export default {
  content: [
    '!**/*_test.go',
    './{build,models,modules,routers,services}/**/*.go',
    './templates/**/*.tmpl',
    './web_src/js/**/*.{ts,js,vue}',
  ].filter(Boolean as unknown as <T>(x: T | boolean) => x is T),
  theme: {
    colors: {
      // make `bg-red` etc work with our CSS variables
      ...Object.fromEntries(vars.filter((prop) => prop.startsWith('color-')).map((prop) => {
        const color = prop.substring(6);
        return [color, `var(--color-${color})`];
      })),
      inherit: 'inherit',
      current: 'currentcolor',
      transparent: 'transparent',
    },
    borderRadius: {
      'none': '0',
      'sm': '2px',
      'DEFAULT': 'var(--border-radius)', // 4px
      'md': 'var(--border-radius-medium)', // 6px
      'lg': '8px',
      'xl': '12px',
      '2xl': '16px',
      '3xl': '24px',
      'full': 'var(--border-radius-full)',
    },
    fontFamily: {
      sans: 'var(--fonts-regular)',
      mono: 'var(--fonts-monospace)',
    },
    fontWeight: {
      light: 'var(--font-weight-light)',
      normal: 'var(--font-weight-normal)',
      medium: 'var(--font-weight-medium)',
      semibold: 'var(--font-weight-semibold)',
      bold: 'var(--font-weight-bold)',
    },
    fontSize: {
      'xs': '11px',
      'sm': '12px',
      'base': '14px', // match body's 1em=14px
      'lg': '16px',
      'xl': '18px',
      ...Object.fromEntries(Array.from({length: 48}, (_, i) => {
        return [String(i), i === 0 ? '0' : `${i}px`];
      })),
    },
    extend: {
      zIndex: {'1': '1'},
    },
  },
} satisfies Config;
