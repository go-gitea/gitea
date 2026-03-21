declare namespace NodeJS {
  interface ProcessEnv {
    GITEA_TEST_E2E_DOMAIN: string;
    GITEA_TEST_E2E_USER: string;
    GITEA_TEST_E2E_EMAIL: string;
    GITEA_TEST_E2E_PASSWORD: string;
    GITEA_TEST_E2E_URL: string;
  }
}
