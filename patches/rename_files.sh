#!/bin/bash

# Iterate through all `.patch` files in the current directory
for file in *.patch; do
  # Extract the full timestamp from the "Date" line
  commit_timestamp=$(grep -m 1 "^Date:" "$file" | sed -E 's/Date: [A-Za-z]+, (.*)/\1/')

  # Convert the extracted timestamp to ISO 8601 format (YYYY-MM-DD_HH-MM-SS)
  iso_timestamp=$(date -d "$commit_timestamp" +"%Y-%m-%d_%H-%M-%S")

  # Prepend the formatted timestamp to the filename
  new_name="${iso_timestamp}_${file}"

  # Rename the file
  mv "$file" "$new_name"

  echo "Renamed $file to $new_name"
done
