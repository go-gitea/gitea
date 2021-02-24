#!/bin/sh

mv ./options/locale/locale_en-US.ini ./options/

# Make sure to only change lines that have the translation enclosed between quotes
sed -i -r -e '/^[a-zA-Z0-9_.-]+[ ]*=[ ]*".*"$/ {
	s/^([a-zA-Z0-9_.-]+)[ ]*="/\1=/
	s/\\"/"/g
	s/"$//
	}' ./options/locale/*.ini

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
