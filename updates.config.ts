import type {Config} from 'updates';

export default {
  exclude: [
    '@mcaptcha/vanilla-glue', // breaking changes in rc versions need to be handled
    'cropperjs', // need to migrate to v2 but v2 is not compatible with v1
    'tailwindcss', // need to migrate
  ],
} satisfies Config;
