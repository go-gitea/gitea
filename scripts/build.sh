#!/usr/bin/env bash
outPath=./output

rm -rf $outPath
mkdir $outPath

go build ../main.go -o gitea
PLATFORM=`uname | cut -d _ -f 1`
if [ $PLATFORM = "MINGW32" ] || [ $PLATFORM = "MINGW64" ] || [ $PLATFORM = "CYGWIN" ]; then
	GOGS_EXE=gogs.exe
else
	GOGS_EXE=gogs
fi
chmod +x $GOGS_EXE
mv $GOGS_EXE $outPath/

cp -r ../conf/ $outPath/conf/
cp -r ../custom/ $outPath/custom/
cp -r dockerfiles/ $outPath/dockerfiles/
cp -r ../public/ $outPath/public/
cp -r ../templates/ $outPath/templates/
cp ../cert.pem $outPath/
cp ../CONTRIBUTING.md $outPath/
cp gogs_supervisord.sh $outPath/
cp ../key.pem $outPath/
cp ../LICENSE $outPath/
cp ../README.md $outPath/
cp ../README_ZH.md $outPath/
cp start.bat $outPath/
cp start.sh $outPath/
cp ../wercker.yml $outPath/
cp mysql.sql $outPath/