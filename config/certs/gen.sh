#!/bin/bash

# Author: Dave Augustus

# Define the directory for OpenSSL
dir="config/certs/openssl"

# Create the directory and navigate into it
mkdir -p $dir && cd $dir

# Generate the CA private key file
openssl genrsa -out ca.key 2048

# Generate the CA x509 certificate file
openssl req -x509 -new -nodes -key ca.key -subj "/CN=themonkeys/C=IN/L=BENGALURU" -days 1825 -out ca.crt

# Create a server private key
openssl genrsa -out server.key 2048

# Create a configuration file for generating the Certificate Signing Request (CSR)
cat > csr.conf << EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
C = IN
ST = Karnataka
L = Bengaluru
O = themonkeys
OU = IT
emailAddress = admin@themonkeys.live
CN = www.themonkeys.live

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = www.themonkeys.live
DNS.2 = themonkeys.live
IP.1 = 192.168.1.100
IP.2 = 192.168.1.101
EOF

# Generate the CSR using the configuration file and the server private key
openssl req -new -key server.key -out server.csr -config csr.conf

# Generate the server certificate using the CSR, the CA private key, and the CA certificate
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 1825 -sha256 -extfile csr.conf -extensions req_ext
