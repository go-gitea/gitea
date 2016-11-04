#!/usr/bin/env bash
outPlattform=linux
outArch=amd64
outPath=./output_$outPlattform_$outArch

rm -rf $outPath
mkdir $outPath

CGO_ENABLED=0 GOOS=$outPlattform GOARCH=$outArch go build ../main.go -o gitea
chmod +x gogs
mv gogs $outPath/

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
