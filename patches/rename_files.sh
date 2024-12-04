#!/bin/bash

# Iterate through all `.patch` files in the current directory
for file in *.patch; do
  # Extract the commit ID from the first line (starts with "From")
  commit_id=$(grep -m 1 "^From " "$file" | awk '{print $2}' | cut -c 1-10)

  # Extract the full timestamp from the "Date" line
  commit_timestamp=$(grep -m 1 "^Date:" "$file" | sed -E 's/Date: [A-Za-z]+, (.*)/\1/')

  # Convert the extracted timestamp to ISO 8601 format (YYYY-MM-DD_HH-MM-SS)
  iso_timestamp=$(date -d "$commit_timestamp" +"%Y-%m-%d_%H-%M-%S")

  # Prepend the formatted timestamp and commit ID to the filename
  new_name="${iso_timestamp}_${commit_id}.patch"

  # Rename the file
  mv "$file" "$new_name"

  echo "Renamed $file to $new_name"
done
