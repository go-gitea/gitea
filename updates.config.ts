import type {Config} from 'updates';

export default {
  exclude: [
    '@mcaptcha/vanilla-glue', // breaking changes in rc versions need to be handled
    'cropperjs', // need to migrate to v2 but v2 is not compatible with v1
    'eslint', // need to migrate to v10
    'tailwindcss', // need to migrate
    '@eslint/json', // needs eslint 10
  ],
} satisfies Config;
