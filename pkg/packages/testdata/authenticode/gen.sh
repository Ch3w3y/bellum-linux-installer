#!/usr/bin/env bash
# Regenerates the Authenticode test fixtures. Needs Go and osslsigncode 2.x.
set -euo pipefail
cd "$(dirname "$0")"
go run gen.go
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
sign() { # out cert extra-args...
  local out=$1 cert=$2; shift 2
  cat "$cert.pem" ca.pem > "$tmp/chain.pem"
  osslsigncode sign -h sha256 -certs "$tmp/chain.pem" -key "$cert.key" \
    -in unsigned.exe -out "$out" "$@" >/dev/null
}
cat tsa.pem ca.pem > "$tmp/tsa-chain.pem"
tsa=(-TSA-certs "$tmp/tsa-chain.pem" -TSA-key tsa.key -TSA-time 1593561600) # 2020-07-01
sign signed.exe signer
sign signed-ts.exe signer "${tsa[@]}"
sign impostor.exe impostor
sign noeku.exe noeku
sign expired.exe expired
sign expired-ts.exe expired "${tsa[@]}"
rm -f ./*.key tsa.pem impostor.pem noeku.pem expired.pem signer.pem
