#!/bin/bash
#
# This script installs the binaries, libs, scripts in the monkey root fs.

source ${MONKEY_SCRIPTS}/common.sh

function installDependencies()
{
    # Install packages:
    # - openssl
    # - wget
    # - go
    # - nginx
    # - opensearch
    # - postgres
    # - migrate
    :;
}
installDependencies

# TODO: Add all files in /the_monkeys to group 'the_monkeys'.

CERTS_PATH="${MONKEY_ROOT}/vault/certs"
CSR_CONFIG_FILE="${CERTS_PATH}/csr.conf"
CERT_CONFIG_FILE="${CERTS_PATH}/cert.conf"
PRIVATE_KEY_FILE="${CERTS_PATH}/prv_key.pem"
CSR_FILE="${CERTS_PATH}/csr.pem"
ROOT_CA_FILE="${CERTS_PATH}/root_ca.pem"
ROOT_CA_KEY_FILE="${CERTS_PATH}/root_ca_key.pem"
CERT_FILE="${CERTS_PATH}/cert.pem"

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

echo "[Installing THE_MONKEYS certs...]"
installCerts

# STEP 2: Install all the services
function installService()
{
    # change these variables to match your setup
    # TODO: SERVICE_NAMEs are all the folder names under services/
    SERVICE_NAME="$1"
    SERVICE_WORKDIR="${MONKEY_ROOT}/bin"
    SERVICE_EXEC="${SERVICE_WORKDIR}/${SERVICE_NAME}"

    # write the service file
    cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
    [Unit]
    Description=$SERVICE_NAME daemon
    [Service]
    User=$MONKEY_USER
    WorkingDirectory=$SERVICE_WORKDIR
    ExecStart=$SERVICE_EXEC
    Restart=always
    RestartSec=5s
    EnvironmentFiles=$MONKEY_ENV_FILE
    [Install]
    WantedBy=multi-user.target
EOF

    # reload systemd manager configuration
    systemctl daemon-reload
    
    # start the service
    systemctl start $SERVICE_NAME
    
    # enable the service to start on boot
    systemctl enable $SERVICE_NAME
}

THE_MONKEYS_SERVICES=(
    "the_monkeys_gateway"
    "the_monkeys_authz"
    "the_monkeys_blog"
    "the_monkeys_users"
    "the_monkeys_storage"
)

# create a new user and group for the monkeys
useradd -r -s /bin/false "$MONKEY_USER"

echo "Setting environment variables..."
echo "TESTENV=irakrigia123" >> $MONKEY_ENV_FILE
echo "[Installing THE_MONKEYS services...]"
for s in "${THE_MONKEYS_SERVICES[@]}"
do
    echo "Installing service: ${s}"
    installService "$s"
done