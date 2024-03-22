CERTS_PATH="./certs"
CSR_CONFIG_FILE="./csr.conf"
CERT_CONFIG_FILE="./cert.conf"
PRIVATE_KEY_FILE="./prv_key.pem"
CSR_FILE="./csr.pem"
ROOT_CA_FILE="./root_ca.pem"
ROOT_CA_KEY_FILE="./root_ca_key.pem"
CERT_FILE="./cert.pem"

# STEP 1: Generate certs and key for TLS.
# /the_monkeys/vault/certs/{cert.pem,prv_key.pem}
# OpenVPN.
function installCerts()
{
    # TODO
    # Check if certificate already exists.
    # Then either leave it or install new certs.
    mkdir -p "$CERTS_PATH"

    cd "$CERTS_PATH"

    # Create a private key.
    openssl genrsa -out prv_key.pem 2048

    # Create a default CSR config file.
    if ! cat > "$CSR_CONFIG_FILE" ; then
        sh_perror "ERROR: failed to write to file"
        exit 1
    fi << EOF
    [ req ]
    default_bits = 2048
    prompt = no
    default_md = sha256
    req_extensions = req_ext
    distinguished_name = dn
    
    [ dn ]
    C = IN
    ST = KARNATAKA
    L = BANGALORE
    O = TheMonkeys
    OU = TheMonkeys-Dev
    CN = themonkeys.life
    
    [ req_ext ]
    subjectAltName = @alt_names
    
    [ alt_names ]
    DNS.1 = themonkeys.life
    DNS.1 = www.themonkeys.life
EOF

    # Generate a CSR
    openssl req -new -key "$PRIVATE_KEY_FILE" -out "$CSR_FILE" -config "$CSR_CONFIG_FILE"

    # Create a default cert config file.
    if ! cat > "$CERT_CONFIG_FILE" ; then
        sh_perror "ERROR: failed to write to file"
        exit 1
    fi << EOF
    authorityKeyIdentifier=keyid,issuer
    basicConstraints=CA:FALSE
    keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
    subjectAltName = @alt_names
    
    [alt_names]
    DNS.1 = themonkeys.life
    DNS.1 = www.themonkeys.life
EOF

    # Create our Root CA
    openssl req -x509 \
        -sha256 -days 356 \
        -nodes \
        -newkey rsa:2048 \
        -subj "/CN=themonkeys.life/C=IN/L=Bangalore" \
        -keyout "$ROOT_CA_KEY_FILE" -out "$ROOT_CA_FILE"

    # Generate the SSL certificate
    openssl x509 -req \
        -in "$CSR_FILE" \
        -CA "$ROOT_CA_FILE" -CAkey "$ROOT_CA_KEY_FILE" \
        -CAcreateserial -out "$CERT_FILE" \
        -days 365 \
        -sha256 -extfile "$CERT_CONFIG_FILE"
}

installCerts()