// Images
local minGoImage = 'golang:1.18';
local goImage = 'golang:1.19';
local xgoImage = 'techknowlogick/xgo:go-1.19.x';
local nodeImage = 'node:18';
local goproxy = 'https://goproxy.io';

// Common objects
local goDepVolume = [
  {
    name: 'deps',
    path: '/go',
  },
];

local platformAMD = {
  arch: 'amd64',
  os: 'linux',
};

local platformARM = {
  arch: 'arm64',
  os: 'linux',
};

// Functions

// end: frontend or backend
local deps(end) = {
  commands: [
    'make deps-' + end,
  ],
  name: 'deps-' + end,
  pull: 'always',
} + if end == 'frontend' then {
  image: nodeImage,
} else {
  image: goImage,
  volumes: goDepVolume,
};

// suffix: blank, version, or branch
// arch: amd64 or arm64
local dockerLinuxRelease(suffix, arch) = {
  depends_on: [
    'testing-amd64',
    'testing-arm64',
  ],
  kind: 'pipeline',
  name: 'docker-linux-' + arch + '-release' + if suffix != '' then '-' + suffix else '',
  platform: if arch == 'amd64' then platformAMD else platformARM,
  steps: [
    {
      commands: [
        'git config --global --add safe.directory /drone/src',
        'git fetch --tags --force',
      ],
      image: 'docker:git',
      name: 'fetch-tags',
      pull: 'always',
    },
    {
      image: 'techknowlogick/drone-docker:latest',
      name: 'publish',
      pull: 'always',
      settings: (if suffix == 'version' then {
                   auto_tag: true,
                   auto_tag_suffix: 'linux-' + arch,
                 } else {
                   auto_tag: false,
                   tags: if suffix == 'branch' then '${DRONE_BRANCH##release/v}-dev-linux-' + arch else 'dev-linux-' + arch,
                 }) + {
        build_args: [
          'GOPROXY=https://goproxy.io',
        ],
        password: {
          from_secret: 'docker_password',
        },
        repo: 'gitea/gitea',
        username: {
          from_secret: 'docker_username',
        },
      },
      when: {
        event: {
          exclude: [
            'pull_request',
          ],
        },
      },
    },
    {
      image: 'techknowlogick/drone-docker:latest',
      name: 'publish-rootless',
      settings: (if suffix == 'version' then {
                   auto_tag: true,
                   auto_tag_suffix: 'linux-' + arch + '-rootless',
                 } else {
                   auto_tag: false,
                   tags: if suffix == 'branch' then '${DRONE_BRANCH##release/v}-dev-linux-' + arch + '-rootless' else 'dev-linux-' + arch + '-rootless',
                 }) + {
        build_args: [
          'GOPROXY=https://goproxy.io',
        ],
        dockerfile: 'Dockerfile.rootless',
        password: {
          from_secret: 'docker_password',
        },
        repo: 'gitea/gitea',
        username: {
          from_secret: 'docker_username',
        },
      },
      when: {
        event: {
          exclude: [
            'pull_request',
          ],
        },
      },
    },
  ],
  trigger: {
    event: {
      exclude: [
        'cron',
      ],
    },
    ref: [
      if suffix == 'version' then 'refs/tags/**' else if suffix == 'branch' then 'refs/heads/release/v*' else 'refs/heads/main',
    ],
  },
  type: 'docker',
};

// version: true or false
local dockerManifest(version) = {
  depends_on: (if version then [
                 'docker-linux-amd64-release-version',
                 'docker-linux-arm64-release-version',
               ] else [
                 'docker-linux-amd64-release',
                 'docker-linux-arm64-release',
                 'docker-linux-amd64-release-branch',
                 'docker-linux-arm64-release-branch',
               ]),
  kind: 'pipeline',
  name: 'docker-manifest' + if version then '-version' else '',
  platform: platformAMD,
  steps: [
    {
      image: 'plugins/manifest',
      name: 'manifest-rootless',
      pull: 'always',
      settings: {
        auto_tag: version,
        ignore_missing: true,
        password: {
          from_secret: 'docker_password',
        },
        spec: 'docker/manifest.rootless.tmpl',
        username: {
          from_secret: 'docker_username',
        },
      },
    },
    {
      image: 'plugins/manifest',
      name: 'manifest',
      settings: {
        auto_tag: version,
        ignore_missing: true,
        password: {
          from_secret: 'docker_password',
        },
        spec: 'docker/manifest.tmpl',
        username: {
          from_secret: 'docker_username',
        },
      },
    },
  ],
  trigger: {
    event: {
      exclude: [
        'cron',
      ],
    },
    ref: (if version then [
            'refs/tags/**',
          ] else [
            'refs/heads/main',
            'refs/heads/release/v*',
          ]),
  },
  type: 'docker',
};

// Pipelines
local compliance = {
  kind: 'pipeline',
  name: 'compliance',
  platform: platformAMD,
  steps: [
    deps('frontend'),
    deps('backend'),
    {
      commands: [
        'make lint-frontend',
      ],
      depends_on: [
        'deps-frontend',
      ],
      image: nodeImage,
      name: 'lint-frontend',
    },
    {
      commands: [
        'make lint-backend',
      ],
      depends_on: [
        'deps-backend',
      ],
      environment: {
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'lint-backend',
      pull: 'always',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make golangci-lint-windows vet',
      ],
      depends_on: [
        'deps-backend',
      ],
      environment: {
        GOARCH: 'amd64',
        GOOS: 'windows',
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'lint-backend-windows',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make lint-backend',
      ],
      depends_on: [
        'deps-backend',
      ],
      environment: {
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TAGS: 'bindata gogit sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'lint-backend-gogit',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make checks-frontend',
      ],
      depends_on: [
        'deps-frontend',
      ],
      image: nodeImage,
      name: 'checks-frontend',
    },
    {
      commands: [
        'make --always-make checks-backend',
      ],
      depends_on: [
        'deps-backend',
      ],
      image: goImage,
      name: 'checks-backend',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make test-frontend',
      ],
      depends_on: [
        'lint-frontend',
      ],
      image: nodeImage,
      name: 'test-frontend',
    },
    {
      commands: [
        'make frontend',
      ],
      depends_on: [
        'deps-frontend',
      ],
      image: nodeImage,
      name: 'build-frontend',
    },
    {
      commands: [
        'go build -o gitea_no_gcc',
      ],
      depends_on: [
        'deps-backend',
        'checks-backend',
      ],
      environment: {
        GO111MODULE: 'on',
        GOPROXY: goproxy,
      },
      image: minGoImage,
      name: 'build-backend-no-gcc',
      pull: 'always',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make backend',
        'rm ./gitea',
      ],
      depends_on: [
        'deps-backend',
        'checks-backend',
      ],
      environment: {
        GO111MODULE: 'on',
        GOARCH: 'arm64',
        GOOS: 'linux',
        GOPROXY: goproxy,
        TAGS: 'bindata gogit',
      },
      image: goImage,
      name: 'build-backend-arm64',
      volumes: goDepVolume,
    },
    {
      commands: [
        'go build -o gitea_windows',
      ],
      depends_on: [
        'deps-backend',
        'checks-backend',
      ],
      environment: {
        GO111MODULE: 'on',
        GOARCH: 'amd64',
        GOOS: 'windows',
        GOPROXY: goproxy,
        TAGS: 'bindata gogit',
      },
      image: goImage,
      name: 'build-backend-windows',
      volumes: goDepVolume,
    },
    {
      commands: [
        'go build -o gitea_linux_386',
      ],
      depends_on: [
        'deps-backend',
        'checks-backend',
      ],
      environment: {
        GO111MODULE: 'on',
        GOARCH: 386,
        GOOS: 'linux',
        GOPROXY: goproxy,
      },
      image: goImage,
      name: 'build-backend-386',
      volumes: goDepVolume,
    },
  ],
  trigger: {
    event: [
      'push',
      'tag',
      'pull_request',
    ],
  },
  type: 'docker',
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
};

local testingAMD64 = {
  depends_on: [
    'compliance',
  ],
  kind: 'pipeline',
  name: 'testing-amd64',
  platform: platformAMD,
  services: [
    {
      environment: {
        MYSQL_ALLOW_EMPTY_PASSWORD: 'yes',
        MYSQL_DATABASE: 'test',
      },
      image: 'mysql:5.7',
      name: 'mysql',
      pull: 'always',
    },
    {
      environment: {
        MYSQL_ALLOW_EMPTY_PASSWORD: 'yes',
        MYSQL_DATABASE: 'testgitea',
      },
      image: 'mysql:8',
      name: 'mysql8',
      pull: 'always',
    },
    {
      environment: {
        ACCEPT_EULA: 'Y',
        MSSQL_PID: 'Standard',
        SA_PASSWORD: 'MwantsaSecurePassword1',
      },
      image: 'mcr.microsoft.com/mssql/server:latest',
      name: 'mssql',
      pull: 'always',
    },
    {
      image: 'gitea/test-openldap:latest',
      name: 'ldap',
      pull: 'always',
    },
    {
      environment: {
        'discovery.type': 'single-node',
      },
      image: 'elasticsearch:7.5.0',
      name: 'elasticsearch',
      pull: 'always',
    },
    {
      commands: [
        'minio server /data',
      ],
      environment: {
        MINIO_ACCESS_KEY: 123456,
        MINIO_SECRET_KEY: 12345678,
      },
      image: 'minio/minio:RELEASE.2021-03-12T00-00-47Z',
      name: 'minio',
      pull: 'always',
    },
  ],
  steps: [
    {
      commands: [
        'git config --global --add safe.directory /drone/src',
        'git fetch --tags --force',
      ],
      image: 'docker:git',
      name: 'fetch-tags',
      pull: 'always',
      when: {
        event: {
          exclude: [
            'pull_request',
          ],
        },
      },
    },
    deps('backend'),
    {
      commands: [
        'git update-ref refs/heads/tag_test ${DRONE_COMMIT_SHA}',
      ],
      image: 'drone/git',
      name: 'tag-pre-condition',
      pull: 'always',
    },
    {
      commands: [
        './build/test-env-prepare.sh',
      ],
      image: 'gitea/test_env:linux-amd64',
      name: 'prepare-test-env',
      pull: 'always',
    },
    {
      commands: [
        './build/test-env-check.sh',
        'make backend',
      ],
      depends_on: [
        'deps-backend',
        'prepare-test-env',
      ],
      environment: {
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'build',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make unit-test-coverage test-check',
      ],
      depends_on: [
        'deps-backend',
        'prepare-test-env',
      ],
      environment: {
        GITHUB_READ_TOKEN: {
          from_secret: 'github_read_token',
        },
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'unit-test',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make unit-test-coverage test-check',
      ],
      depends_on: [
        'deps-backend',
        'prepare-test-env',
      ],
      environment: {
        GITHUB_READ_TOKEN: {
          from_secret: 'github_read_token',
        },
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata gogit sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'unit-test-gogit',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make test-mysql-migration integration-test-coverage',
      ],
      depends_on: [
        'build',
      ],
      environment: {
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata',
        TEST_INDEXER_CODE_ES_URL: 'http://elastic:changeme@elasticsearch:9200',
        TEST_LDAP: 1,
        USE_REPO_TEST_DIR: 1,
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'test-mysql',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'timeout -s ABRT 50m make test-mysql8-migration test-mysql8',
      ],
      depends_on: [
        'build',
      ],
      environment: {
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata',
        TEST_LDAP: 1,
        USE_REPO_TEST_DIR: 1,
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'test-mysql8',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make test-mssql-migration test-mssql',
      ],
      depends_on: [
        'build',
      ],
      environment: {
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata',
        TEST_LDAP: 1,
        USE_REPO_TEST_DIR: 1,
      },
      image: 'gitea/test_env:linux-amd64',
      name: 'test-mssql',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'make coverage',
      ],
      depends_on: [
        'unit-test',
        'test-mysql',
      ],
      environment: {
        GOPROXY: goproxy,
        TAGS: 'bindata',
      },
      image: goImage,
      name: 'generate-coverage',
      when: {
        branch: [
          'main',
        ],
        event: [
          'push',
          'pull_request',
        ],
      },
    },
    {
      depends_on: [
        'generate-coverage',
      ],
      image: 'woodpeckerci/plugin-codecov:next-alpine',
      name: 'coverage-codecov',
      pull: 'always',
      settings: {
        files: [
          'coverage.all',
        ],
        token: {
          from_secret: 'codecov_token',
        },
      },
      when: {
        branch: [
          'main',
        ],
        event: [
          'push',
          'pull_request',
        ],
      },
    },
  ],
  trigger: {
    event: [
      'push',
      'tag',
      'pull_request',
    ],
  },
  type: 'docker',
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
};

local testingARM64 = {
  depends_on: [
    'compliance',
  ],
  kind: 'pipeline',
  name: 'testing-arm64',
  platform: platformARM,
  services: [
    {
      environment: {
        POSTGRES_DB: 'test',
        POSTGRES_PASSWORD: 'postgres',
      },
      image: 'postgres:10',
      name: 'pgsql',
      pull: 'default',
    },
    {
      image: 'gitea/test-openldap:latest',
      name: 'ldap',
      pull: 'default',
    },
  ],
  steps: [
    {
      commands: [
        'git config --global --add safe.directory /drone/src',
        'git fetch --tags --force',
      ],
      image: 'docker:git',
      name: 'fetch-tags',
      pull: 'always',
      when: {
        event: {
          exclude: [
            'pull_request',
          ],
        },
      },
    },
    deps('backend'),
    {
      commands: [
        './build/test-env-prepare.sh',
      ],
      image: 'gitea/test_env:linux-arm64',
      name: 'prepare-test-env',
      pull: 'always',
    },
    {
      commands: [
        './build/test-env-check.sh',
        'make backend',
      ],
      depends_on: [
        'deps-backend',
        'prepare-test-env',
      ],
      environment: {
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TAGS: 'bindata gogit sqlite sqlite_unlock_notify',
      },
      image: 'gitea/test_env:linux-arm64',
      name: 'build',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'timeout -s ABRT 50m make test-sqlite-migration test-sqlite',
      ],
      depends_on: [
        'build',
      ],
      environment: {
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata gogit sqlite sqlite_unlock_notify',
        TEST_TAGS: 'gogit sqlite sqlite_unlock_notify',
        USE_REPO_TEST_DIR: 1,
      },
      image: 'gitea/test_env:linux-arm64',
      name: 'test-sqlite',
      user: 'gitea',
      volumes: goDepVolume,
    },
    {
      commands: [
        'timeout -s ABRT 50m make test-pgsql-migration test-pgsql',
      ],
      depends_on: [
        'build',
      ],
      environment: {
        GOPROXY: goproxy,
        RACE_ENABLED: true,
        TAGS: 'bindata gogit',
        TEST_LDAP: 1,
        TEST_TAGS: 'gogit',
        USE_REPO_TEST_DIR: 1,
      },
      image: 'gitea/test_env:linux-arm64',
      name: 'test-pgsql',
      user: 'gitea',
      volumes: goDepVolume,
    },
  ],
  trigger: {
    event: [
      'push',
      'tag',
      'pull_request',
    ],
  },
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
};

local testinge2e = {
  depends_on: [
    'compliance',
  ],
  kind: 'pipeline',
  name: 'testing-e2e',
  platform: platformAMD,
  services: [
    {
      environment: {
        POSTGRES_DB: 'testgitea-e2e',
        POSTGRES_INITDB_ARGS: "--encoding=UTF8 --lc-collate='en_US.UTF-8' --lc-ctype='en_US.UTF-8'",
        POSTGRES_PASSWORD: 'postgres',
      },
      image: 'postgres:10',
      name: 'pgsql',
      pull: 'default',
    },
  ],
  steps: [
    deps('frontend'),
    {
      commands: [
        'make frontend',
      ],
      depends_on: [
        'deps-frontend',
      ],
      image: nodeImage,
      name: 'build-frontend',
    },
    deps('backend'),
    {
      commands: [
        'curl -sLO https://go.dev/dl/go1.19.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.19.linux-amd64.tar.gz',
        'groupadd --gid 1001 gitea && useradd -m --gid 1001 --uid 1001 gitea',
        'apt-get -qq update && apt-get -qqy install build-essential',
        "export TEST_PGSQL_SCHEMA=''",
        './build/test-env-prepare.sh',
        'su gitea bash -c "export PATH=$PATH:/usr/local/go/bin && timeout -s ABRT 40m make test-e2e-pgsql"',
      ],
      depends_on: [
        'build-frontend',
        'deps-backend',
      ],
      environment: {
        DEBIAN_FRONTEND: 'noninteractive',
        GOPROXY: goproxy,
        GOSUMDB: 'sum.golang.org',
        TEST_PGSQL_DBNAME: 'testgitea-e2e',
        USE_REPO_TEST_DIR: 1,
      },
      image: 'mcr.microsoft.com/playwright:v1.28.0-focal',
      name: 'test-e2e',
      volumes: goDepVolume,
    },
  ],
  trigger: {
    event: [
      'pull_request',
    ],
  },
  type: 'docker',
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
};

local updatingTranslations = {
  kind: 'pipeline',
  name: 'update_translations',
  platform: platformARM,
  steps: [
    {
      environment: {
        CROWDIN_KEY: {
          from_secret: 'crowdin_key',
        },
      },
      image: 'jonasfranz/crowdin',
      name: 'download',
      pull: 'always',
      settings: {
        download: true,
        export_dir: 'options/locale/',
        ignore_branch: true,
        project_identifier: 'gitea',
      },
    },
    {
      commands: [
        './build/update-locales.sh',
      ],
      image: 'alpine:3.17',
      name: 'update',
      pull: 'always',
    },
    {
      environment: {
        DRONE_COMMIT_AUTHOR: 'GiteaBot',
        DRONE_COMMIT_AUTHOR_EMAIL: 'teabot@gitea.io',
        GIT_PUSH_SSH_KEY: {
          from_secret: 'git_push_ssh_key',
        },
      },
      image: 'appleboy/drone-git-push',
      name: 'push',
      pull: 'always',
      settings: {
        author_email: 'teabot@gitea.io',
        author_name: 'GiteaBot',
        branch: 'main',
        commit: true,
        commit_message: '[skip ci] Updated translations via Crowdin',
        remote: 'git@github.com:go-gitea/gitea.git',
      },
    },
    {
      environment: {
        CROWDIN_KEY: {
          from_secret: 'crowdin_key',
        },
      },
      image: 'jonasfranz/crowdin',
      name: 'upload_translations',
      pull: 'always',
      settings: {
        files: {
          'locale_en-US.ini': 'options/locale/locale_en-US.ini',
        },
        ignore_branch: true,
        project_identifier: 'gitea',
      },
    },
  ],
  trigger: {
    branch: [
      'main',
    ],
    cron: [
      'update_translations',
    ],
    event: [
      'cron',
    ],
  },
};

local updateGitignoreAndLicenses = {
  kind: 'pipeline',
  name: 'update_gitignore_and_licenses',
  platform: platformARM,
  steps: [
    {
      commands: [
        'timeout -s ABRT 40m make generate-license generate-gitignore',
      ],
      image: goImage,
      name: 'download',
      pull: 'always',
    },
    {
      environment: {
        DRONE_COMMIT_AUTHOR: 'GiteaBot',
        DRONE_COMMIT_AUTHOR_EMAIL: 'teabot@gitea.io',
        GIT_PUSH_SSH_KEY: {
          from_secret: 'git_push_ssh_key',
        },
      },
      image: 'appleboy/drone-git-push',
      name: 'push',
      pull: 'always',
      settings: {
        author_email: 'teabot@gitea.io',
        author_name: 'GiteaBot',
        branch: 'main',
        commit: true,
        commit_message: '[skip ci] Updated licenses and gitignores',
        remote: 'git@github.com:go-gitea/gitea.git',
      },
    },
  ],
  trigger: {
    branch: [
      'main',
    ],
    cron: [
      'update_gitignore_and_licenses',
    ],
    event: [
      'cron',
    ],
  },
  type: 'docker',
};

local releaseLatest = {
  depends_on: [
    'testing-amd64',
    'testing-arm64',
  ],
  kind: 'pipeline',
  name: 'release-latest',
  platform: platformAMD,
  steps: [
    {
      commands: [
        'git config --global --add safe.directory /drone/src',
        'git fetch --tags --force',
      ],
      image: 'docker:git',
      name: 'fetch-tags',
      pull: 'always',
    },
    deps('frontend'),
    deps('backend'),
    {
      commands: [
        'curl -sL https://deb.nodesource.com/setup_16.x | bash - && apt-get -qqy install nodejs',
        'export PATH=$PATH:$GOPATH/bin',
        'make release',
      ],
      environment: {
        DEBIAN_FRONTEND: 'noninteractive',
        GOPROXY: goproxy,
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: xgoImage,
      name: 'static',
      pull: 'always',
      volumes: goDepVolume,
    },
    {
      environment: {
        GPGSIGN_KEY: {
          from_secret: 'gpgsign_key',
        },
        GPGSIGN_PASSPHRASE: {
          from_secret: 'gpgsign_passphrase',
        },
      },
      image: 'plugins/gpgsign:1',
      name: 'gpg-sign',
      pull: 'always',
      settings: {
        detach_sign: true,
        excludes: [
          'dist/release/*.sha256',
        ],
        files: [
          'dist/release/*',
        ],
      },
    },
    {
      environment: {
        AWS_ACCESS_KEY_ID: {
          from_secret: 'aws_access_key_id',
        },
        AWS_SECRET_ACCESS_KEY: {
          from_secret: 'aws_secret_access_key',
        },
      },
      image: 'woodpeckerci/plugin-s3:latest',
      name: 'release-branch',
      pull: 'always',
      settings: {
        acl: 'public-read',
        bucket: 'gitea-artifacts',
        endpoint: 'https://ams3.digitaloceanspaces.com',
        path_style: true,
        source: 'dist/release/*',
        strip_prefix: 'dist/release/',
        target: '/gitea/${DRONE_BRANCH##release/v}',
      },
      when: {
        branch: [
          'release/*',
        ],
        event: [
          'push',
        ],
      },
    },
    {
      environment: {
        AWS_ACCESS_KEY_ID: {
          from_secret: 'aws_access_key_id',
        },
        AWS_SECRET_ACCESS_KEY: {
          from_secret: 'aws_secret_access_key',
        },
      },
      image: 'woodpeckerci/plugin-s3:latest',
      name: 'release-main',
      settings: {
        acl: 'public-read',
        bucket: 'gitea-artifacts',
        endpoint: 'https://ams3.digitaloceanspaces.com',
        path_style: true,
        source: 'dist/release/*',
        strip_prefix: 'dist/release/',
        target: '/gitea/main',
      },
      when: {
        branch: [
          'main',
        ],
        event: [
          'push',
        ],
      },
    },
  ],
  trigger: {
    branch: [
      'main',
      'release/*',
    ],
    event: [
      'push',
    ],
  },
  type: 'docker',
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
  workspace: {
    base: '/source',
    path: '/',
  },
};

local releaseVersion = {
  depends_on: [
    'testing-arm64',
    'testing-amd64',
  ],
  kind: 'pipeline',
  name: 'release-version',
  platform: platformAMD,
  steps: [
    {
      commands: [
        'git config --global --add safe.directory /drone/src',
        'git fetch --tags --force',
      ],
      image: 'docker:git',
      name: 'fetch-tags',
      pull: 'always',
    },
    deps('frontend'),
    deps('backend'),
    {
      commands: [
        'curl -sL https://deb.nodesource.com/setup_16.x | bash - && apt-get -qqy install nodejs',
        'export PATH=$PATH:$GOPATH/bin',
        'make release',
      ],
      depends_on: [
        'fetch-tags',
      ],
      environment: {
        DEBIAN_FRONTEND: 'noninteractive',
        GOPROXY: goproxy,
        TAGS: 'bindata sqlite sqlite_unlock_notify',
      },
      image: xgoImage,
      name: 'static',
      pull: 'always',
      volumes: goDepVolume,
    },
    {
      depends_on: [
        'static',
      ],
      environment: {
        GPGSIGN_KEY: {
          from_secret: 'gpgsign_key',
        },
        GPGSIGN_PASSPHRASE: {
          from_secret: 'gpgsign_passphrase',
        },
      },
      image: 'plugins/gpgsign:1',
      name: 'gpg-sign',
      pull: 'always',
      settings: {
        detach_sign: true,
        excludes: [
          'dist/release/*.sha256',
        ],
        files: [
          'dist/release/*',
        ],
      },
    },
    {
      depends_on: [
        'gpg-sign',
      ],
      environment: {
        AWS_ACCESS_KEY_ID: {
          from_secret: 'aws_access_key_id',
        },
        AWS_SECRET_ACCESS_KEY: {
          from_secret: 'aws_secret_access_key',
        },
      },
      image: 'woodpeckerci/plugin-s3:latest',
      name: 'release-tag',
      pull: 'always',
      settings: {
        acl: 'public-read',
        bucket: 'gitea-artifacts',
        endpoint: 'https://ams3.digitaloceanspaces.com',
        path_style: true,
        source: 'dist/release/*',
        strip_prefix: 'dist/release/',
        target: '/gitea/${DRONE_TAG##v}',
      },
    },
    {
      depends_on: [
        'gpg-sign',
      ],
      environment: {
        GITHUB_TOKEN: {
          from_secret: 'github_token',
        },
      },
      image: 'plugins/github-release:latest',
      name: 'github',
      pull: 'always',
      settings: {
        file_exists: 'overwrite',
        files: [
          'dist/release/*',
        ],
      },
    },
  ],
  trigger: {
    event: [
      'tag',
    ],
  },
  volumes: [
    {
      name: 'deps',
      temp: {},
    },
  ],
  workspace: {
    base: '/source',
    path: '/',
  },
};

local docs = {
  depends_on: [
    'compliance',
  ],
  kind: 'pipeline',
  name: 'docs',
  platform: platformARM,
  steps: [
    {
      commands: [
        'apk add --no-cache make bash curl',
        'cd docs',
        'make trans-copy clean build',
      ],
      image: 'plugins/hugo:latest',
      name: 'build-docs',
      pull: 'always',
    },
    {
      environment: {
        NETLIFY_TOKEN: {
          from_secret: 'netlify_token',
        },
      },
      image: 'techknowlogick/drone-netlify:latest',
      name: 'publish-docs',
      pull: 'always',
      settings: {
        path: 'docs/public/',
        site_id: 'd2260bae-7861-4c02-8646-8f6440b12672',
      },
      when: {
        branch: [
          'main',
        ],
        event: [
          'push',
        ],
      },
    },
  ],
  trigger: {
    event: [
      'push',
      'tag',
      'pull_request',
    ],
  },
  type: 'docker',
};

local dockerLinuxarm64DryRun = {
  depends_on: [
    'compliance',
  ],
  kind: 'pipeline',
  name: 'docker-linux-arm64-dry-run',
  platform: platformARM,
  steps: [
    {
      environment: {
        PLUGIN_MIRROR: {
          from_secret: 'plugin_mirror',
        },
      },
      image: 'techknowlogick/drone-docker:latest',
      name: 'dryrun',
      pull: 'always',
      settings: {
        build_args: [
          'GOPROXY=https://goproxy.io',
        ],
        dry_run: true,
        repo: 'gitea/gitea',
        tags: 'linux-arm64',
      },
      when: {
        event: [
          'pull_request',
        ],
      },
    },
  ],
  trigger: {
    ref: [
      'refs/pull/**',
    ],
  },
  type: 'docker',
};

local notifications = {
  clone: {
    disable: true,
  },
  depends_on: [
    'testing-amd64',
    'testing-arm64',
    'release-version',
    'release-latest',
    'docker-linux-amd64-release',
    'docker-linux-arm64-release',
    'docker-linux-amd64-release-version',
    'docker-linux-arm64-release-version',
    'docker-linux-amd64-release-branch',
    'docker-linux-arm64-release-branch',
    'docker-manifest',
    'docker-manifest-version',
    'docs',
  ],
  kind: 'pipeline',
  name: 'notifications',
  platform: platformARM,
  steps: [
    {
      image: 'appleboy/drone-discord:1.2.4',
      name: 'discord',
      pull: 'always',
      settings: {
        message: '{{#success build.status}} ✅  Build #{{build.number}} of `{{repo.name}}` succeeded.\n\n📝 Commit by {{commit.author}} on `{{commit.branch}}`:\n``` {{commit.message}} ```\n\n🌐 {{ build.link }} {{else}} ❌  Build #{{build.number}} of `{{repo.name}}` failed.\n\n📝 Commit by {{commit.author}} on `{{commit.branch}}`:\n``` {{commit.message}} ```\n\n🌐 {{ build.link }} {{/success}}\n',
        webhook_id: {
          from_secret: 'discord_webhook_id',
        },
        webhook_token: {
          from_secret: 'discord_webhook_token',
        },
      },
    },
  ],
  trigger: {
    branch: [
      'main',
      'release/*',
    ],
    event: [
      'push',
      'tag',
    ],
    status: [
      'success',
      'failure',
    ],
  },
  type: 'docker',
};

// Output
[
  compliance,
  testingAMD64,
  testingARM64,
  testinge2e,
  updatingTranslations,
  updateGitignoreAndLicenses,
  releaseLatest,
  releaseVersion,
  docs,
  dockerLinuxRelease('version', 'amd64'),
  dockerLinuxRelease('', 'amd64'),
  dockerLinuxRelease('branch', 'amd64'),
  dockerLinuxarm64DryRun,
  dockerLinuxRelease('version', 'arm64'),
  dockerLinuxRelease('', 'arm64'),
  dockerLinuxRelease('branch', 'arm64'),
  dockerManifest(true),
  dockerManifest(false),
  notifications,
]
