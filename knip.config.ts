import type {KnipConfig} from 'knip';

export default {
  entry: [
    '*.ts',
    'tools/*.ts',
  ],
  project: [
    'web_src/**/*.{ts,vue}',
  ],
  ignoreDependencies: [
    // dependencies used in Makefile or tools
    '@primer/octicons',
    'markdownlint-cli',
    'nolyfill',
    'spectral-cli-bundle',
    'vue-tsc',
    'webpack-cli',
  ],
} satisfies KnipConfig;
