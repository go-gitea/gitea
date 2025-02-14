#!/bin/sh

# this script runs in alpine image which only has `sh` shell

set +e
if sed --version 2>/dev/null | grep -q GNU; then
  SED_INPLACE="sed -i"
else
  SED_INPLACE="sed -i ''"
fi
set -e

if [ ! -f ./options/locale/locale_en-US.ini ]; then
  echo "please run this script in the root directory of the project"
  exit 1
fi

mv ./options/locale/locale_en-US.ini ./options/

# the "ini" library for locale has many quirks, its behavior is different from Crowdin.
# see i18n_test.go for more details

# this script helps to unquote the Crowdin outputs for the quirky ini library
# * find all `key="...\"..."` lines
# * remove the leading quote
# * remove the trailing quote
# * unescape the quotes
# * eg: key="...\"..." => key=..."...
$SED_INPLACE -r -e '/^[-.A-Za-z0-9_]+[ ]*=[ ]*".*"$/ {
	s/^([-.A-Za-z0-9_]+)[ ]*=[ ]*"/\1=/
	s/"$//
	s/\\"/"/g
	}' ./options/locale/*.ini

# * if the escaped line is incomplete like `key="...` or `key=..."`, quote it with backticks
# * eg: key="... => key=`"...`
# * eg: key=..." => key=`..."`
$SED_INPLACE -r -e 's/^([-.A-Za-z0-9_]+)[ ]*=[ ]*(".*[^"])$/\1=`\2`/' ./options/locale/*.ini
$SED_INPLACE -r -e 's/^([-.A-Za-z0-9_]+)[ ]*=[ ]*([^"].*")$/\1=`\2`/' ./options/locale/*.ini

# Remove translation under 25% of en_us
baselines=$(wc -l "./options/locale_en-US.ini" | cut -d" " -f1)
baselines=$((baselines / 4))
for filename in ./options/locale/*.ini; do
  lines=$(wc -l "$filename" | cut -d" " -f1)
  if [ $lines -lt $baselines ]; then
    echo "Removing $filename: $lines/$baselines"
    rm "$filename"
  fi
done

mv ./options/locale_en-US.ini ./options/locale/
