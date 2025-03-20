name: e2e-tests

on:
  pull_request:

concurrency:
  group: ${{ github.workflow }}-${{ github.head_ref || github.run_id }}
  cancel-in-progress: true

jobs:
  files-changed:
    uses: ./.github/workflows/files-changed.yml

  test-e2e:
    if: needs.files-changed.outputs.backend == 'true' || needs.files-changed.outputs.frontend == 'true' || needs.files-changed.outputs.actions == 'true'
    needs: files-changed
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: package-lock.json
      - run: make deps-frontend frontend deps-backend
      - run: npx playwright install --with-deps
      - run: make test-e2e-sqlite
        timeout-minutes: 40
        env:
          USE_REPO_TEST_DIR: 1
