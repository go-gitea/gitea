#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  enroll.sh --url URL --username NAME [options]

Options:
  --internal-token TOKEN     Optional internal token (required only if server enforces it)
  --internal-token-file PATH Optional path to token file (single line)
  --full-name NAME            Optional display name
  --email EMAIL               Optional contact email; server can generate placeholder
  --machine-id ID             Optional machine identity (e.g. whoami@hostname)
  --network-id ID             Optional network identity (e.g. 10.0.0.12)
  --owner-agent true|false    Default: false
  --token-name NAME           Optional PAT name
  --token-scopes CSV          Optional CSV scopes (default: public-only,read:repository)
EOF
}

url=""
internal_token=""
internal_token_file=""
username=""
full_name=""
email=""
machine_id=""
network_id=""
owner_agent="false"
token_name=""
token_scopes=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url) url="${2:-}"; shift 2 ;;
    --internal-token) internal_token="${2:-}"; shift 2 ;;
    --internal-token-file) internal_token_file="${2:-}"; shift 2 ;;
    --username) username="${2:-}"; shift 2 ;;
    --full-name) full_name="${2:-}"; shift 2 ;;
    --email) email="${2:-}"; shift 2 ;;
    --machine-id) machine_id="${2:-}"; shift 2 ;;
    --network-id) network_id="${2:-}"; shift 2 ;;
    --owner-agent) owner_agent="${2:-}"; shift 2 ;;
    --token-name) token_name="${2:-}"; shift 2 ;;
    --token-scopes) token_scopes="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$url" || -z "$username" ]]; then
  usage
  exit 1
fi

if [[ -z "$internal_token" && -n "$internal_token_file" ]]; then
  if [[ ! -f "$internal_token_file" ]]; then
    echo "internal token file not found: $internal_token_file" >&2
    exit 1
  fi
  internal_token="$(head -n 1 "$internal_token_file" | tr -d '\r\n')"
fi

if [[ "$owner_agent" != "true" && "$owner_agent" != "false" ]]; then
  echo "--owner-agent must be true or false" >&2
  exit 1
fi

scope_json='[]'
if [[ -n "$token_scopes" ]]; then
  IFS=',' read -r -a scope_array <<<"$token_scopes"
  scope_json='['
  for i in "${!scope_array[@]}"; do
    scope="${scope_array[$i]}"
    scope="${scope#"${scope%%[![:space:]]*}"}"
    scope="${scope%"${scope##*[![:space:]]}"}"
    [[ -n "$scope" ]] || continue
    if [[ "$scope_json" != "[" ]]; then
      scope_json+=","
    fi
    scope_json+="\"$scope\""
  done
  scope_json+=']'
fi

payload="{\"username\":\"$username\",\"owner_agent\":$owner_agent"
if [[ -n "$full_name" ]]; then
  payload+=",\"full_name\":\"$full_name\""
fi
if [[ -n "$email" ]]; then
  payload+=",\"email\":\"$email\""
fi
if [[ -n "$machine_id" ]]; then
  payload+=",\"machine_identity\":\"$machine_id\""
fi
if [[ -n "$network_id" ]]; then
  payload+=",\"network_identifier\":\"$network_id\""
fi
if [[ -n "$token_name" ]]; then
  payload+=",\"token_name\":\"$token_name\""
fi
if [[ "$scope_json" != "[]" ]]; then
  payload+=",\"token_scopes\":$scope_json"
fi
payload+="}"

curl_args=(
  -fsS
  -H "Content-Type: application/json"
  -X POST "$url/api/v1/agents/enroll"
  -d "$payload"
)
if [[ -n "$internal_token" ]]; then
  curl_args+=(-H "X-Internal-Token: $internal_token")
fi
curl "${curl_args[@]}"
echo
