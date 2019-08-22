## [1.9.2](https://github.com/go-gitea/gitea/releases/tag/v1.9.2) - 2019-08-22
* BUGFIXES
  * Fix wrong sender when send slack webhook (#7918) (#7924)
  * Upload support text/plain; charset=utf8 (#7899)
  *  lfs/lock: round locked_at timestamp to second (#7872) (#7875)
  * fix non existent milestone with 500 error (#7867) (#7873)
* ENHANCEMENT
  * Fix pull creation with empty changes (#7920) (#7926)
* BUILD
  * drone/docker: prepare multi-arch release + provide arm64 image (#7571) (#7884)
