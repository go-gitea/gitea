export default {
  rootDir: 'web_src',
  setupFilesAfterEnv: ['jest-extended/all'],
  testEnvironment: 'jsdom',
  testMatch: ['<rootDir>/**/*.test.js'],
  testTimeout: 20000,
  transform: {},
  verbose: false,
};

