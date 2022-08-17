export default {
  rootDir: 'web_src',
  setupFiles: ['<rootDir>/js/testUtils/setup.js'],
  setupFilesAfterEnv: ['jest-extended/all'],
  testEnvironment: '@happy-dom/jest-environment',
  testMatch: ['<rootDir>/**/*.test.js'],
  testTimeout: 20000,
  transform: {
    '\\.svg$': '<rootDir>/js/testUtils/jestRawLoader.js',
  },
  verbose: false,
};

