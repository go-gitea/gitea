import {readFileSync} from 'node:fs';
import {env} from 'node:process';
import {parse} from 'css-variables-parser';

const isProduction = env.NODE_ENV !== 'development';

export default {
  prefix: 'tw-',
  important: true, // the frameworks are mixed together, so tailwind needs to override other framework's styles
  content: [
    isProduction && '!./templates/devtest/**/*',
    isProduction && '!./web_src/js/standalone/devtest.js',
    './templates/**/*.tmpl',
    './web_src/**/*.{js,vue}',
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
      ...Object.fromEntries(
        Object.keys(parse([
          readFileSync(new URL('web_src/css/themes/theme-gitea-light.css', import.meta.url), 'utf8'),
          readFileSync(new URL('web_src/css/themes/theme-gitea-dark.css', import.meta.url), 'utf8'),
        ].join('\n'), {})).filter((prop) => prop.startsWith('color-')).map((prop) => {
          const color = prop.substring(6);
          return [color, `var(--color-${color})`];
        })
      ),
      inherit: 'inherit',
      current: 'currentcolor',
      transparent: 'transparent',
    },
  },
};
