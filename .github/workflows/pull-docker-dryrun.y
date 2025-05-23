name: docker-dryrun

on:
  pull_request:

concurrency:
  group: ${{ github.workflow }}-${{ github.head_ref || github.run_id }}
  cancel-in-progress: true

jobs:
  files-changed:
    uses: ./.github/workflows/files-changed.yml

  regular:
    if: needs.files-changed.outputs.docker == 'true' || needs.files-changed.outputs.actions == 'true'
    needs: files-changed
    runs-on: ubuntu-latest
    steps:
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v5
        with:
          push: false
          tags: gitea/gitea:linux-amd64

  rootless:
    if: needs.files-changed.outputs.docker == 'true' || needs.files-changed.outputs.actions == 'true'
    needs: files-changed
    runs-on: ubuntu-latest
    steps:
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v5
        with:
          push: false
          file: Dockerfile.rootless
          tags: gitea/gitea:linux-amd64
