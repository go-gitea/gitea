#!/bin/bash

gitea_version=1.0.1
tested_gogs_version="0.9.114.1227"
gogs_binary=gogs
gitea_binary=gitea
download_gitea=true
gitea_path=

function usage() { 
  echo "Optional parameters: [-b Gitea binary] [-i Gitea install dir] [-o gogs binary] [-h help]";
  exit 1; 
}

while getopts ":b::i:o:h:" opt; do
  case $opt in
    b)
        gitea_binary=${OPTARG}
        download_gitea=false
      ;;
    i)
        gitea_path=${OPTARG}
      ;;
    o)
        gogs_binary=${OPTARG}
      ;;
    h)
       usage
      ;;
    \?)
      echo -e "Invalid option: -$OPTARG" 
      exit 1
      ;;
    :)
      usage
      exit 1
      ;;
  esac
done

function exitOnError() {
  if [ "$?" != "0" ]; then
      echo -e $1
      exit 1
  fi
}

function checkBinary() {
  if [ ! -f $1 ]; then
   echo "Unable to find $1"
   exit 1
  fi
}

function continueYN(){
  while true; do
    echo -e "$1 Yes or No"
    read yn
    case $yn in
        [Yy]* ) break;;
        [Nn]* ) exit 1;;
        * )     echo "Please answer yes or no.";;
    esac
  done
}

########## Binary checks
if pidof "$gogs_binary" >/dev/null; then
 echo "Please stop gogs before migrating to Gitea"
 exit 1
fi

checkBinary "$gogs_binary"

if [ ! -x "$gogs_binary" ]; then
 echo "Please make sure that you are running this script as the gogs user"
 exit 1
fi

########## Version check
gogs_version=$(./$gogs_binary --version)
original_IFS=$IFS
IFS="." && current_version=(${gogs_version#"Gogs version "}) && minimal_version=($tested_gogs_version)
IFS=$original_IFS

count=0
for i in "${current_version[@]}" 
do
  if [ $i -gt ${minimal_version[$count]} ]; then
   echo -e "!!!--WARNING--!!!\nYour $gogs_version is newer than the tested Gogs version $tested_gogs_version\nUse this script on your own risk\n!!!--WARNING--!!!"
   break
  fi
  let count+=1
done

########## Disclaimer
continueYN "This migration script creates a backup before it starts with the actual migration
If something goes wrong you could always resotre this backup.
The backups are stored into your gogs folder in gogs-dump-[timestamp].zip file

Migrating from gogs to gitea, are you sure?"

########## gogs dump
echo "Creating a backup of gogs, this could take a while..."
./"$gogs_binary" dump
exitOnError "Failed to create a gogs dump"

########## Create Gitea folder
if [ -z "$gitea_path" ]; then
  echo "Where do you want to install Gitea?"
  read gitea_path 
fi

if [ ! -d "$gitea_path" ]; then
  mkdir -p "$gitea_path"
  exitOnError
fi

if [ "$(ls -A $gitea_path)" ]; then
  continueYN "!!!--WARNING--!!!\nDirectory $gitea_path is not empty, do you want to continue?"
fi


########## Download Gitea
if [ $download_gitea == true ]; then

  ########## Detect os
  case "$OSTYPE" in
    darwin*)    platform="darwin-10.6";; 
    linux*)     platform="linux" ;;
    freebsd*)   platform="bsd" ;;
    netbsd*)    platform="bsd" ;;
    openbsd*)   platform="bsd" ;;
    *)          echo "Unsupported os: $OSTYPE\n Please download/compile your own binary and run this script with the -b option" exit 1;;
  esac

  arch=""
  bits=""
  if [[ "$platform" == "linux" ]] || [[ "$platform" == "bsd" ]]; then
    arch="$(uname -m | sed -e 's/arm\(.*\)/arm-\1/' -e s/aarch64.*/arm64/)"
  fi

  if [[ "$platform" == "bsd" ]] && [[ "$arch" != "arm"* ]]; then
    echo "Currently Gitea only supports arm prebuilt binarys on bsd"
    exit 1
  fi

  if [[ "$arch" != "arm"* ]] &&  [[ "$arch" != "mips"* ]]; then
    arch=""
    case "$(getconf LONG_BIT)" in
      64*)  bits="amd64";; 
      32*)  bits="386" ;;
    esac
  fi

  ########## Wget Gitea
  echo "Downloading Gitea"
  file="gitea-$gitea_version-$platform-$arch$bits"
  url="https://dl.gitea.io/gitea/$gitea_version/$file"
  wget "$url" -P "$gitea_path"
  exitOnError "Failed to download $url"

  wget "$url.sha256" -P "$gitea_path"
  exitOnError "Failed to Gitea checksum $url.sha256"

  echo "Comparing checksums"
  gogs_dir=$(pwd)
  cd "$gitea_path"

  sha256sum -c "$file.sha256"
  exitOnError "Downloaded Gitea checksums do not match"

  rm "$file.sha256"
  mv "$file" gitea
  cd "$gogs_dir"

else
  checkBinary "$gitea_binary"
  if [ "$gitea_binary" != "$gitea_path/gitea" ];then
    cp "$gitea_binary" "$gitea_path/gitea"
  fi
fi

########## Copy gogs data to Gitea folder
echo "Copying gogs data to Gitea, this could take a while..."
cp -R custom "$gitea_path"
cp -R data "$gitea_path"
#cp -R conf "$gitea_path"

########## Moving & deleting old files
#mv $gitea_path/conf $gitea_path/options
cd "$gitea_path"
mv "custom/conf/app.ini" "custom/conf/gogs_app.ini"
url="https://raw.githubusercontent.com/go-gitea/gitea/v$gitea_version/conf/app.ini"
wget "$url" -P "custom/conf/"
exitOnError "Unable to download Gitea app.ini"
rm -f conf/README.md

echo -e "Migration is almost complete, you only need to merge custom/conf/gogs_app.ini into custom/conf/app.ini"
continueYN "Do you want to start Gitea?"

########## Starting Gitea
echo "Starting Gitea"
chmod +x gitea
./gitea web
exitOnError "Failed to start Gitea"