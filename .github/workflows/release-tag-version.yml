name: release-tag-version

on:
  push:
    tags:
      - "v1.*"
      - "!v1*-rc*"
      - "!v1*-dev"

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false

jobs:
  binary:
    runs-on: namespace-profile-gitea-release-binary
    steps:
      - uses: actions/checkout@v4
      # fetch all commits instead of only the last as some branches are long lived and could have many between versions
      # fetch all tags to ensure that "git describe" reports expected Gitea version, eg. v1.21.0-dev-1-g1234567
      - run: git fetch --unshallow --quiet --tags --force
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: package-lock.json
      - run: make deps-frontend deps-backend
      # xgo build
      - run: make release
        env:
          TAGS: bindata sqlite sqlite_unlock_notify
      - name: import gpg key
        id: import_gpg
        uses: crazy-max/ghaction-import-gpg@v6
        with:
          gpg_private_key: ${{ secrets.GPGSIGN_KEY }}
          passphrase: ${{ secrets.GPGSIGN_PASSPHRASE }}
      - name: sign binaries
        run: |
          for f in dist/release/*; do
            echo '${{ secrets.GPGSIGN_PASSPHRASE }}' | gpg --pinentry-mode loopback --passphrase-fd 0 --batch --yes --detach-sign -u ${{ steps.import_gpg.outputs.fingerprint }} --output "$f.asc" "$f"
          done
      # clean branch name to get the folder name in S3
      - name: Get cleaned branch name
        id: clean_name
        run: |
          REF_NAME=$(echo "${{ github.ref }}" | sed -e 's/refs\/heads\///' -e 's/refs\/tags\/v//' -e 's/release\/v//')
          echo "Cleaned name is ${REF_NAME}"
          echo "branch=${REF_NAME}" >> "$GITHUB_OUTPUT"
      - name: configure aws
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-region: ${{ secrets.AWS_REGION }}
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
      - name: upload binaries to s3
        run: |
          aws s3 sync dist/release s3://${{ secrets.AWS_S3_BUCKET }}/gitea/${{ steps.clean_name.outputs.branch }} --no-progress
      - name: Install GH CLI
        uses: dev-hanz-ops/install-gh-cli-action@v0.1.0
        with:
          gh-cli-version: 2.39.1
      - name: create github release
        run: |
          gh release create ${{ github.ref_name }} --title ${{ github.ref_name }} --notes-from-tag dist/release/*
        env:
          GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}
  docker-rootful:
    runs-on: namespace-profile-gitea-release-docker
    steps:
      - uses: actions/checkout@v4
      # fetch all commits instead of only the last as some branches are long lived and could have many between versions
      # fetch all tags to ensure that "git describe" reports expected Gitea version, eg. v1.21.0-dev-1-g1234567
      - run: git fetch --unshallow --quiet --tags --force
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: gitea/gitea
          # this will generate tags in the following format:
          # latest
          # 1
          # 1.2
          # 1.2.3
          tags: |
            type=semver,pattern={{major}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{version}}
      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - name: build rootful docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
  docker-rootless:
    runs-on: namespace-profile-gitea-release-docker
    steps:
      - uses: actions/checkout@v4
      # fetch all commits instead of only the last as some branches are long lived and could have many between versions
      # fetch all tags to ensure that "git describe" reports expected Gitea version, eg. v1.21.0-dev-1-g1234567
      - run: git fetch --unshallow --quiet --tags --force
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: gitea/gitea
          # each tag below will have the suffix of -rootless
          flavor: |
            suffix=-rootless,onlatest=true
          # this will generate tags in the following format (with -rootless suffix added):
          # latest
          # 1
          # 1.2
          # 1.2.3
          tags: |
            type=semver,pattern={{major}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{version}}
      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - name: build rootless docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          file: Dockerfile.rootless
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
