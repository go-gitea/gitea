import {readFileSync} from 'node:fs';
import {parse} from 'css-variables-parser';

const colors = Object.keys(parse([
  readFileSync(new URL('web_src/css/themes/theme-gitea-light.css', import.meta.url), 'utf8'),
  readFileSync(new URL('web_src/css/themes/theme-gitea-dark.css', import.meta.url), 'utf8'),
].join('\n'), {})).filter((prop) => prop.startsWith('color-')).map((prop) => prop.substring(6));

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './templates/**/*.tmpl',
    './web_src/**/*.{js,vue}',
  ],
  blocklist: [
    // classes that conflict with fomantic
    'table', 'inline', 'grid', 'truncate', 'transition', 'fixed',
    // classes that don't work without CSS variables from "@tailwind base" which we don't use
    'transform', 'shadow', 'ring', 'blur', 'grayscale', 'invert', '!invert', 'filter', '!filter',
    'backdrop-filter',
    // false-positives or otherwise unneeded
    '[-a-zA-Z:0-9_.]',
  ],
  theme: {
    colors: {
      // make bg-red etc work with our CSS variables
      ...Object.fromEntries(colors.map((color) => [color, `var(--color-${color})`])),
      inherit: 'inherit',
      currentcolor: 'currentcolor',
      transparent: 'transparent',
    },
  },
};
