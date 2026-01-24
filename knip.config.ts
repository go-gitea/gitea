import type {KnipConfig} from 'knip';

export default {
  entry: [
    '*.ts',
    'tools/*.ts',
  ],
  project: [
    'web_src/**/*.{ts,vue}',
  ],
  // dependencies used in Makefile or tools
  ignoreDependencies: [
    '@primer/octicons',
    'markdownlint-cli',
    'nolyfill',
    'spectral-cli-bundle',
    'vue-tsc',
    'webpack-cli',
  ],
} satisfies KnipConfig;
