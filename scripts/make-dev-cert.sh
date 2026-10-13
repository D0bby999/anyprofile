#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Creates a self-signed code-signing identity "AnyProfile Dev" in the login
# keychain. `make build` signs with it, so the app keeps one identity across
# rebuilds and macOS remembers "Always Allow" for its Keychain items instead of
# asking again after every build (an ad hoc signature is just the file hash).
#
# Run once per machine. macOS asks for your login password once, to trust the
# certificate for code signing.
set -eu

NAME="AnyProfile Dev"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
OPENSSL=/usr/bin/openssl

if security find-identity -v -p codesigning | grep -q "\"$NAME\""; then
	echo "$NAME already exists"
	exit 0
fi

tmp=$(mktemp -d)
chmod 700 "$tmp"
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/cert.cnf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $NAME
[ext]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
EOF

# genrsa writes the traditional RSA PEM that `security import -f openssl` reads.
$OPENSSL genrsa -out "$tmp/key.pem" 2048 2>/dev/null
$OPENSSL req -new -x509 -days 3650 -sha256 -config "$tmp/cert.cnf" \
	-key "$tmp/key.pem" -out "$tmp/cert.pem"

# The key goes straight into the keychain; only codesign may use it unprompted.
security import "$tmp/key.pem" -k "$KEYCHAIN" -t priv -f openssl -T /usr/bin/codesign >/dev/null
security import "$tmp/cert.pem" -k "$KEYCHAIN" -t cert >/dev/null
# User-level trust for code signing only; this is the step that asks for the password.
security add-trusted-cert -r trustRoot -p codeSign -k "$KEYCHAIN" "$tmp/cert.pem"

security find-identity -v -p codesigning | grep "\"$NAME\""
