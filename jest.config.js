export default {
  setupFilesAfterEnv: ['jest-extended'],
  testTimeout: 20000,
  testMatch: [
    '**/web_src/**/*.test.js',
  ],
  transform: {},
  verbose: false,
};

