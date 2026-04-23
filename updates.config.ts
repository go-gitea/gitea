import type {Config} from 'updates';

export default {
  pin: {
    '@mcaptcha/vanilla-glue': '^0.1', // breaking changes in rc versions need to be handled
    'cropperjs': '^1', // need to migrate to v2 but v2 is not compatible with v1
    'tailwindcss': '^3', // need to migrate
  },
} satisfies Config;
