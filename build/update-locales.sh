#!/bin/sh

# this script runs in alpine image which only has `sh` shell
if [ ! -f ./options/locale/locale_en-US.json ]; then
  echo "please run this script in the root directory of the project"
  exit 1
fi

mv ./options/locale/locale_en-US.json ./options/

# Remove translation under 25% of en_us
baselines=$(wc -l "./options/locale_en-US.json" | cut -d" " -f1)
baselines=$((baselines / 4))
for filename in ./options/locale/*.json; do
  lines=$(wc -l "$filename" | cut -d" " -f1)
  if [ $lines -lt $baselines ]; then
    echo "Removing $filename: $lines/$baselines"
    rm "$filename"
  fi
done

mv ./options/locale_en-US.json ./options/locale/
