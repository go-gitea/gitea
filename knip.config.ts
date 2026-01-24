import type {KnipConfig} from 'knip';

export default {
  project: ['web_src/**/*.{ts,vue}'],
  exclude: ['dependencies', 'devDependencies'],
} satisfies KnipConfig;
