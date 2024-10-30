#!/bin/bash
#
# This script uninstalls everything related to THE_MONKEYS.

source ${MONKEY_SCRIPTS}/common.sh

CERTS_PATH="${MONKEY_ROOT}/vault/certs"
CSR_CONFIG_FILE="${CERTS_PATH}/csr.conf"
CERT_CONFIG_FILE="${CERTS_PATH}/cert.conf"
PRIVATE_KEY_FILE="${CERTS_PATH}/prv_key.pem"
CSR_FILE="${CERTS_PATH}/csr.pem"
ROOT_CA_FILE="${CERTS_PATH}/root_ca.pem"
ROOT_CA_KEY_FILE="${CERTS_PATH}/root_ca_key.pem"
CERT_FILE="${CERTS_PATH}/cert.pem"

function uninstallCerts()
{
    rm -rf "$CERTS_PATH"
}
uninstallCerts

function uninstallService()
{
    # change these variables to match your setup
    # TODO: SERVICE_NAMEs are all the folder names under services/
    SERVICE_NAME="$1"
    SERVICE_WORKDIR="${MONKEY_ROOT}/bin"
    SERVICE_EXEC="${SERVICE_WORKDIR}/${SERVICE_NAME}"

    # start the service
    systemctl stop $SERVICE_NAME

    # enable the service to start on boot
    systemctl disable $SERVICE_NAME

    echo "rm -f /etc/systemd/system/$SERVICE_NAME.service"
    rm -f /etc/systemd/system/$SERVICE_NAME.service

    # reload systemd manager configuration
    systemctl daemon-reload

    echo "rm -f $SERVICE_EXEC"
    rm -f "$SERVICE_EXEC"

    echo "rm -rf $MONKEY_ETC"
    rm -rf "$MONKEY_ETC"
}

THE_MONKEYS_SERVICES=(
    "the_monkeys_gateway"
    "the_monkeys_authz"
    "the_monkeys_blog"
    "the_monkeys_users"
    "the_monkeys_storage"
)

echo "[Installing THE_MONKEYS services...]"
for s in "${THE_MONKEYS_SERVICES[@]}"
do
    echo "Uninstalling service: ${s}"
    uninstallService "$s"
done

echo "Deleting MONKEY_ROOT"
rm -rf "$MONKEY_ROOT"

echo "Uninstall successful."