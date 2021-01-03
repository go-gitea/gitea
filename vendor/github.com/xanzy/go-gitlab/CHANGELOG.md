go-github CHANGELOG
===================

0.6.0
-----
- Add support for the V4 Gitlab API. This means the older V3 API is no longer fully supported
  with this version. If you still need that version, please use the `f-api-v3` branch.

0.4.0
-----
- Add support to use [`sudo`](https://docs.gitlab.com/ce/api/README.html#sudo) for all API calls.
- Add support for the Notification Settings API.
- Add support for the Time Tracking API.
- Make sure that the error response correctly outputs any returned errors.
- And a reasonable number of smaller enhanchements and bugfixes.

0.3.0
-----
- Moved the tags related API calls to their own service, following the Gitlab API structure.

0.2.0
-----
- Convert all Option structs to use pointers for their fields.

0.1.0
-----
- Initial release.
