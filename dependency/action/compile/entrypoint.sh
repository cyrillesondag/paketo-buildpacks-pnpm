#!/usr/bin/env bash

set -eu
set -o pipefail

function main() {
  local version output_dir target temp_dir os arch

  # default values
  os="linux"
  arch="x64"

  while [ "${#}" != 0 ]; do
    case "${1}" in
      --version)
        version="${2}"
        shift 2
        ;;

      --output-dir)
        output_dir="${2}"
        shift 2
        ;;

      --target)
        target="${2}"
        shift 2
        ;;

       --os)
        os="${2}"
        shift 2
        ;;

      --arch)
        arch="${2}"
        shift 2
        ;;

      "")
        shift
        ;;

      *)
        echo "unknown argument \"${1}\""
        exit 1
    esac
  done

  if [[ -z "${version:-}" ]]; then
    echo "--version is required"
    exit 1
  fi

  if [[ -z "${output_dir:-}" ]]; then
    echo "--output-dir is required"
    exit 1
  fi

  if [[ -z "${target:-}" ]]; then
    echo "--target is required"
    exit 1
  fi

  case "${arch}" in
    x64|amd64)
      arch="x64"
      ;;
    arm64)
      arch="arm64"
      ;;
    *)
      echo "unsupported architecture \"${arch}\""
      exit 1
      ;;
  esac

  temp_dir="$(mktemp -d)"

  pushd "${temp_dir}"
    echo "Downloading pnpm binaries ${version}"

    mkdir "bin"

    curl "https://github.com/pnpm/pnpm/releases/download/v${version}/pnpm-linux-${arch}" \
      --silent \
      --output "${temp_dir}/bin/pnpm"

    chmod +x

    tar zcvf "${output_dir}/temp.tgz" .
  popd

  pushd "${output_dir}"

    SHA256=$(sha256sum temp.tgz)
    SHA256="${SHA256:0:64}"

    OUTPUT_TARBALL_NAME="pnpm_${version}_${os}_${arch}_${target}_${SHA256:0:8}.tgz"
    OUTPUT_SHAFILE_NAME="pnpm_${version}_${os}_${arch}_${target}_${SHA256:0:8}.tgz.checksum"

    echo "Building tarball ${OUTPUT_TARBALL_NAME}"

    mv temp.tgz "${OUTPUT_TARBALL_NAME}"

    echo "Creating checksum file for ${OUTPUT_TARBALL_NAME}"
    echo "sha256:${SHA256}" > "${OUTPUT_SHAFILE_NAME}"
  popd
}

main "${@:-}"