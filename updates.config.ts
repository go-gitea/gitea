import type {Config} from 'updates';

export default {
  exclude: [
    '@mcaptcha/vanilla-glue', // breaking changes in rc versions need to be handled
    'cropperjs', // need to migrate to v2 but v2 is not compatible with v1
    'tailwindcss', // need to migrate
    'typescript', // wait on https://github.com/typescript-eslint/typescript-eslint/issues/12123
  ],
} satisfies Config;
