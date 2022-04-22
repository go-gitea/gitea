export default {
  rootDir: 'web_src',
  setupFilesAfterEnv: ['jest-extended/all'],
  testEnvironment: 'jsdom',
  testMatch: ['<rootDir>/**/*.test.js'],
  testTimeout: 20000,
  transform: {
    '\\.svg$': 'jest-raw-loader',
  },
  verbose: false,
};

