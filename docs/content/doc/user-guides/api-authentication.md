# API user guide

## User

### /users/{username}/tokens

Basic usage:

```
$ curl --request GET --url https://m:mypassword@git.your.host/api/v1/users/m/tokens
[{"name":"test","sha1":"..."},{"name":"dev","sha1":"..."}]
```
