#!/usr/bin/env bash
# Copyright (c) Lapse Technologies, Inc.
# SPDX-License-Identifier: MPL-2.0
#
# Register a released version of terraform-provider-clickstack into a Terrakube
# private registry. Terrakube stores only metadata; the actual artifacts stay
# hosted on the GitHub release. This script reads the release's SHA256SUMS and
# creates the provider, the version, and one implementation per os/arch.
#
# Run it from anywhere that can reach BOTH the Terrakube API and github.com
# (typically from inside the cluster, or via a port-forward / tunnel).
#
# Prereqs: bash, curl, jq, gpg (with the signing public key available).
#
# Required env:
#   TERRAKUBE_URL        Base URL of the Terrakube API (e.g. https://terrakube-api.example.com)
#   TERRAKUBE_TOKEN      Terrakube Personal Access Token (Bearer)
#   TERRAKUBE_ORG_ID     Terrakube organization UUID (the {organizationId} in the API path)
#   VERSION              Provider version WITHOUT the leading 'v' (e.g. 0.1.0)
#   GPG_FINGERPRINT      Fingerprint of the GPG key the release was signed with
#
# Optional env:
#   PROVIDER_NAME        Provider type name in Terrakube (default: clickstack)
#   GH_REPO              owner/repo hosting the release (default: parsed from `git remote`)
#   PROTOCOLS            Terraform plugin protocol version (default: 5.0)
#   PROVIDER_ID          Skip provider creation and reuse this id (for re-runs)
#   TERRAKUBE_ORG_NAME   Org *name* (only used to print the required_providers source line)
#   TERRAKUBE_HOST       Registry host (only used to print the required_providers source line)
set -euo pipefail

: "${TERRAKUBE_URL:?set TERRAKUBE_URL}"
: "${TERRAKUBE_TOKEN:?set TERRAKUBE_TOKEN}"
: "${TERRAKUBE_ORG_ID:?set TERRAKUBE_ORG_ID}"
: "${VERSION:?set VERSION (without leading v, e.g. 0.1.0)}"
: "${GPG_FINGERPRINT:?set GPG_FINGERPRINT}"

PROVIDER_NAME="${PROVIDER_NAME:-clickstack}"
PROTOCOLS="${PROTOCOLS:-5.0}"
PROJECT="terraform-provider-${PROVIDER_NAME}"

if [[ -z "${GH_REPO:-}" ]]; then
  origin="$(git config --get remote.origin.url)"
  GH_REPO="$(printf '%s' "$origin" | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')"
fi

base="https://github.com/${GH_REPO}/releases/download/v${VERSION}"
shasums_url="${base}/${PROJECT}_${VERSION}_SHA256SUMS"
shasums_sig_url="${shasums_url}.sig"

api="${TERRAKUBE_URL%/}/api/v1/organization/${TERRAKUBE_ORG_ID}/provider"
auth=(-H "Authorization: Bearer ${TERRAKUBE_TOKEN}" -H "Content-Type: application/vnd.api+json")

# GPG public key material Terrakube needs to verify the signature.
key_id="$(printf '%s' "${GPG_FINGERPRINT: -16}" | tr '[:lower:]' '[:upper:]')"
ascii_armor="$(gpg --armor --export "${GPG_FINGERPRINT}")"
[[ -n "${ascii_armor}" ]] || { echo "ERROR: empty GPG public key for ${GPG_FINGERPRINT}" >&2; exit 1; }

echo ">> Fetching ${shasums_url}"
shasums="$(curl -fsSL "${shasums_url}")"

# 1) Provider
if [[ -z "${PROVIDER_ID:-}" ]]; then
  echo ">> Creating provider '${PROVIDER_NAME}'"
  body="$(jq -nc --arg name "${PROVIDER_NAME}" \
    '{data:{type:"provider",attributes:{name:$name,description:"ClickStack (HyperDX) provider"}}}')"
  resp="$(curl -fsS "${auth[@]}" -X POST "${api}" -d "${body}")"
  PROVIDER_ID="$(printf '%s' "${resp}" | jq -r '.data.id')"
fi
echo "   providerId=${PROVIDER_ID}"

# 2) Version
echo ">> Creating version ${VERSION} (protocols ${PROTOCOLS})"
body="$(jq -nc --arg v "${VERSION}" --arg p "${PROTOCOLS}" \
  '{data:{type:"version",attributes:{versionNumber:$v,protocols:$p}}}')"
resp="$(curl -fsS "${auth[@]}" -X POST "${api}/${PROVIDER_ID}/version" -d "${body}")"
VERSION_ID="$(printf '%s' "${resp}" | jq -r '.data.id')"
echo "   versionId=${VERSION_ID}"

# 3) One implementation per os/arch, read straight from SHA256SUMS.
while read -r shasum file; do
  [[ "${file}" == *.zip ]] || continue
  platform="${file#${PROJECT}_${VERSION}_}"   # -> os_arch.zip
  platform="${platform%.zip}"
  os="${platform%%_*}"
  arch="${platform##*_}"

  echo ">> Implementation ${os}/${arch} (${file})"
  body="$(jq -nc \
    --arg os "${os}" --arg arch "${arch}" --arg filename "${file}" \
    --arg downloadUrl "${base}/${file}" \
    --arg shasumsUrl "${shasums_url}" --arg shasumsSignatureUrl "${shasums_sig_url}" \
    --arg shasum "${shasum}" --arg keyId "${key_id}" --arg asciiArmor "${ascii_armor}" \
    '{data:{type:"implementation",attributes:{
        os:$os, arch:$arch, filename:$filename,
        downloadUrl:$downloadUrl, shasumsUrl:$shasumsUrl, shasumsSignatureUrl:$shasumsSignatureUrl,
        shasum:$shasum, keyId:$keyId, asciiArmor:$asciiArmor,
        trustSignature:"", source:"", sourceUrl:""}}}')"
  curl -fsS "${auth[@]}" -X POST "${api}/${PROVIDER_ID}/version/${VERSION_ID}/implementation" -d "${body}" >/dev/null
done <<< "${shasums}"

echo
echo "Done. Reference it in Terraform with:"
if [[ -n "${TERRAKUBE_HOST:-}" && -n "${TERRAKUBE_ORG_NAME:-}" ]]; then
  cat <<EOF
  terraform {
    required_providers {
      ${PROVIDER_NAME} = {
        source  = "${TERRAKUBE_HOST}/${TERRAKUBE_ORG_NAME}/${PROVIDER_NAME}"
        version = "${VERSION}"
      }
    }
  }
EOF
else
  echo "  source = \"<TERRAKUBE_HOST>/<ORG_NAME>/${PROVIDER_NAME}\", version = \"${VERSION}\""
fi
