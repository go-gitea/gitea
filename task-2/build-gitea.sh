#!/bin/bash

set -eu

echo "Gitea Local Build and Run Script"

#check if required tools are installed

req_tools=("go" "node" "pnpm" "make" "python3")

for tool in "${req_tools[@]}"; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "Error - Required tool not found: $tool"
		exit 1
	fi
done
echo "All required tools are installed."

#Displaying dependencies version

echo "Checking dependencies versions"
echo "Go: $(go version)"
echo "Node.js: $(node -v)"
echo "pnpm: $(pnpm -v)"
echo "Make: $(make -v | head -n 1)"

#check the correct project dir

echo "checking the gitea project directory"

if  [ "$#" -ne 1 ]; then
	echo "Error - Gitea project dir not provided."
	echo "Usage; $0 <gitea-proj-dir>"
	exit 1
fi

gitea_dir="$1"

if [ ! -d "$gitea_dir" ]; then
	echo "Error - directory doesn't exist: $gitea_dir"
	exit 1
fi

if [ ! -f "$gitea_dir/go.mod" ] || [ ! -f "$gitea_dir/Makefile" ]; then
	echo "Error - dir not there in gitea project."
	exit 1
fi

echo "Gitea project directory checked: $gitea_dir"

#build from source

echo "Building the gitea from source"

cd "$gitea_dir"

if ! make build; then
	echo "Error - Gitea build failure"
	exit 1
fi

echo "Gitea build successful"

#verify gitea binary

echo "verifying the gitea binary"

if [ ! -f "$gitea_dir/gitea" ]; then
       echo "Error - Gitea binary not found"
       exit 1
fi

echo "Gitea binary created"

#check port 3000
port=3000
echo "Check whether port `3000` is already in use"

if ss -ltn | grep -q :"$port "; then
	echo "Error - Port $port is already in use"
	exit 1
fi

echo "Port $port is available"

#Display the local URL to verify gitea

echo "Starting gitea web server"
echo "URL: http://localhost:$port"

"$gitea_dir/gitea" web --port "$port"
