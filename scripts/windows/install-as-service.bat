@ECHO off

:: This script relies on nssm.exe to work.
:: Please, download it and make it available on the system path,
:: or copy it to the gogs path.
:: https://nssm.cc/download
:: This script itself should run in the gogs path, too.
:: In case of startup failure, please read carefully the log file.
:: Make sure Gitea work running manually with "gitea web" before running
:: this script.
:: And, please, read carefully the installation docs first:
:: https://gogs.io/docs/installation
:: To unistall the service, run "nssm remove gogs" and restart Windows.

:: Set the folder where you extracted Gitea. Omit the last slash.
SET gogspath=C:\gogs

nssm install gogs "%gogspath%\gogs.exe"
nssm set gogs AppParameters "web"
nssm set gogs Description "A painless self-hosted Git service."
nssm set gogs DisplayName "Gitea - Git with a cup of tea"
nssm set gogs Start SERVICE_DELAYED_AUTO_START
nssm set gogs AppStdout "%gogspath%\gogs.log"
nssm start gogs
pause
