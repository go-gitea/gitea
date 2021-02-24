# gitea-vet

[![Build Status](https://drone.gitea.com/api/badges/gitea/gitea-vet/status.svg)](https://drone.gitea.com/gitea/gitea-vet)

`go vet` tool for Gitea

| Analyzer   | Description                                                                 |
|------------|-----------------------------------------------------------------------------|
| Imports    | Checks for import sorting. stdlib->code.gitea.io->other                     |
| License    | Checks file headers for some form of `Copyright...YYYY...Gitea/Gogs`        |
| Migrations | Checks for black-listed packages in `code.gitea.io/gitea/models/migrations` |
