import {readFileSync} from 'node:fs';
import {env} from 'node:process';
import {parse} from 'postcss';

const isProduction = env.NODE_ENV !== 'development';

function extractRootVars(css) {
  const root = parse(css);
  const vars = new Set();
  root.walkRules((rule) => {
    if (rule.selector !== ':root') return;
    rule.each((decl) => {
      if (decl.value && decl.prop.startsWith('--')) {
        vars.add(decl.prop.substring(2));
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
  prefix: 'tw-',
  important: true, // the frameworks are mixed together, so tailwind needs to override other framework's styles
  content: [
    isProduction && '!./templates/devtest/**/*',
    isProduction && '!./web_src/js/standalone/devtest.js',
    '!./templates/swagger/v1_json.tmpl',
    '!./templates/user/auth/oidc_wellknown.tmpl',
    '!**/*_test.go',
    '!./modules/{public,options,templates}/bindata.go',
    './{build,models,modules,routers,services}/**/*.go',
    './templates/**/*.tmpl',
    './web_src/js/**/*.{js,vue}',
  ].filter(Boolean),
  blocklist: [
    // classes that don't work without CSS variables from "@tailwind base" which we don't use
    'transform', 'shadow', 'ring', 'blur', 'grayscale', 'invert', '!invert', 'filter', '!filter',
    'backdrop-filter',
    // unneeded classes
    '[-a-zA-Z:0-9_.]',
  ],
  theme: {
    colors: {
      // make `tw-bg-red` etc work with our CSS variables
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
      'full': 'var(--border-radius-circle)', // 50%
    },
  },
};
