import type {KnipConfig} from 'knip';

export default {
  entry: [
    'web_src/js/index.ts',
    'web_src/js/features/eventsource.sharedworker.ts',
    'web_src/js/standalone/devtest.ts',
    'web_src/js/standalone/external-render-iframe.ts',
    'web_src/js/standalone/swagger.ts',
  ],
  project: ['web_src/**/*.{ts,vue}'],
  ignore: ['updates.config.ts'],
  exclude: ['dependencies', 'devDependencies'],
  webpack: false, // avoid error "EsbuildPlugin is not a constructor", likely a bug in knip
} satisfies KnipConfig;
