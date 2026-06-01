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
  important: true, // the frameworks are mixed together, so tailwind needs to override other framework's styles
  content: [
    '!./templates/swagger/v1_json.tmpl',
    '!./templates/user/auth/oidc_wellknown.tmpl',
    '!**/*_test.go',
    './{build,models,modules,routers,services}/**/*.go',
    './templates/**/*.tmpl',
    './web_src/js/**/*.{ts,js,vue}',
  ].filter(Boolean as unknown as <T>(x: T | boolean) => x is T),
  blocklist: [
    // disabled on purpose: Gitea styles shadows/transforms/filters with its own CSS and does not use Tailwind's
    'transform', 'shadow', 'ring', 'blur', 'grayscale', 'invert', '!invert', 'filter', '!filter',
    'backdrop-filter',
    // we use double-class .hidden.hidden defined in web_src/css/helpers.css for increased specificity
    'hidden',
    // unneeded classes
    '[-a-zA-Z:0-9_.]',
  ],
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
    fontSize: { // rarely used, but "text-base" (matching body's 1em=14px) is useful to reset font-size in a header container
      'xs': '11px',
      'sm': '12px',
      'base': '14px',
      'lg': '18px',
      'xl': '20px',
      '2xl': '24px',
      '3xl': '30px',
      '4xl': '36px',
      '5xl': '48px',
      '6xl': '60px',
      '7xl': '72px',
      '8xl': '96px',
      '9xl': '128px',
      ...Object.fromEntries(Array.from({length: 100}, (_, i) => {
        return [`${i}`, `${i === 0 ? '0' : `${i}px`}`];
      })),
    },
    extend: {
      zIndex: {'1': '1'},
    },
  },
} satisfies Config;
