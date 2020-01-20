#!/bin/sh

mv ./options/locale/locale_en-US.ini ./options/
sed -i -e 's/=\"/=/g' -e 's/\"$$//g' ./options/locale/*.ini
sed -i -e 's/\\\\\\\\\"/\"/g' ./options/locale/*.ini

# Remove translation under 25% of en_us
baselines=`wc -l "./options/locale_en-US.ini" | cut -d" " -f1`
baselines=$((baselines / 4))
for filename in ./options/locale/*.ini; do
  lines=`wc -l "$filename" | cut -d" " -f1`
  if [ $lines -lt $baselines ]; then
    echo "Removing $filename: $lines/$baselines"
    rm "$filename"
  fi
done

mv ./options/locale_en-US.ini ./options/locale/
