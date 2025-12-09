// there could be different "testing" concepts, for example: backend's "setting.IsInTesting"
// even if backend is in testing mode, frontend could be complied in production mode
// so this function only checks if the frontend is in unit testing mode (usually from *.test.ts files)
export function isInFrontendUnitTest() {
  return process.env.TEST === 'true';
}
