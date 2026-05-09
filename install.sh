#!/bin/sh
set -e

REPO="deveshpharswan/stackup"
INSTALL_DIR="${STACKUP_INSTALL_DIR:-/usr/local/bin}"

main() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
    esac

    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        *) echo "Unsupported OS: $os (use install.ps1 for Windows)" >&2; exit 1 ;;
    esac

    echo "Detecting platform: ${os}/${arch}"

    # Get latest release tag
    tag=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
    if [ -z "$tag" ]; then
        echo "Error: could not determine latest release" >&2
        exit 1
    fi
    version="${tag#v}"
    echo "Latest version: ${version}"

    # Download
    filename="stackup_${version}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${tag}/${filename}"
    echo "Downloading ${url}..."

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    curl -sSfL -o "${tmpdir}/${filename}" "$url"

    # Extract
    tar -xzf "${tmpdir}/${filename}" -C "$tmpdir"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/stackup" "${INSTALL_DIR}/stackup"
    else
        echo "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmpdir}/stackup" "${INSTALL_DIR}/stackup"
    fi

    chmod +x "${INSTALL_DIR}/stackup"
    echo "Installed stackup ${version} to ${INSTALL_DIR}/stackup"
    echo ""
    echo "Run 'stackup version' to verify."
}

main
