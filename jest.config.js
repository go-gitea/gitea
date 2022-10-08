// to run tests with ES6 module, node must run with "--experimental-vm-modules", or see Makefile's "test-frontend" for reference
export default {
  rootDir: 'web_src',
  setupFilesAfterEnv: ['jest-extended/all'],
  testEnvironment: 'jest-environment-jsdom',
  testMatch: ['<rootDir>/**/*.test.js'],
  testTimeout: 20000,
  transform: {
    '\\.svg$': '<rootDir>/js/testUtils/jestRawLoader.js',
  },
  setupFiles: [
    './js/testUtils/jestSetup.js', // prepare global variables used by our code (eg: window.config)
  ],
  verbose: false,
};
