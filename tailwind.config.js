import {readFileSync} from 'node:fs';
import {env} from 'node:process';
import {parse} from 'postcss';
import plugin from 'tailwindcss/plugin.js';

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
    // we use double-class tw-hidden defined in web_src/css/helpers.css for increased specificity
    'hidden',
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
    fontSize: { // not using `rem` units because our root is currently 14px
      'xs': '12px',
      'sm': '14px',
      'base': '16px',
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
  },
  plugins: [
    plugin(({addUtilities}) => {
      addUtilities({
        // tw-hidden must win all other "display: xxx !important" classes to get the chance to "hide" an element.
        // do not use:
        // * "[hidden]" attribute: it's too weak, can not be applied to an element with "display: flex"
        // * ".hidden" class: it has been polluted by Fomantic UI in many cases
        // * inline style="display: none": it's difficult to tweak
        // * jQuery's show/hide/toggle: it can not show/hide elements with "display: xxx !important"
        // only use:
        // * this ".tw-hidden" class
        // * showElem/hideElem/toggleElem functions in "utils/dom.js"
        '.hidden.hidden': {
          'display': 'none',
        },
        // proposed class from https://github.com/tailwindlabs/tailwindcss/pull/12128
        '.break-anywhere': {
          'overflow-wrap': 'anywhere',
        },
      });
    }),
  ],
};
