export default {
  setupFilesAfterEnv: ['jest-extended'],
  testTimeout: 10000,
  testPathIgnorePatterns: [
    '/node_modules/',
    '/public/',
    '/vendor/',
  ],
  transform: {},
  verbose: false,
};

