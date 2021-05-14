export default {
  setupFilesAfterEnv: ['jest-extended'],
  testTimeout: 20000,
  rootDir: 'web_src',
  testMatch: [
    '<rootDir>/**/*.test.js',
  ],
  transform: {},
  verbose: false,
};

