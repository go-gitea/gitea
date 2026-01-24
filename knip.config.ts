import type {KnipConfig} from 'knip';

export default {
  project: ['web_src/**/*.{ts,vue}'],
  ignore: ['updates.config.ts'],
  exclude: ['dependencies', 'devDependencies'],
} satisfies KnipConfig;
