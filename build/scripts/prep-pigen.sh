#!/bin/bash

PIGEN_BASEDIR="../../RPi-Distro/pi-gen/"
PIGEN_FILESDIR="${PIGEN_BASEDIR}/stage2/05-simtezilo/files"

function usage() {
    echo "Usage: $0 [-s suffix] [-r release] <channel>"
    exit 1
}

release="false"
suffix=""

while getopts "hrs:" Option
do
  case $Option in
    r ) release="true";;
    s ) suffix=$OPTARG;;
    * ) usage;;
  esac
done

shift $(($OPTIND - 1))

if [ "$#" -ne 1 ]; then
    usage
fi

channel=$1
version=$(cat VERSION)

if [ -n "$suffix" ]; then
    version="${version}-${suffix}"
fi


function setupPiGen() {
    # make release

    tarfile="./dist/releases/${channel}/${version}/simtezilo-${version}-linux-arm64.tar.gz"

    if [ ! -f "${tarfile}" ]; then
        echo "Error: Release tarball not found: ${tarfile}"
        exit 1
    fi

    tar -zxf "${tarfile}" -C ${PIGEN_FILESDIR}/

    cp ./data/tmp/default-m1.conf ${PIGEN_FILESDIR}/default.conf

    releaseDate=$(stat -f "%Sm" -t "%Y-%m-%dT%H:%M:%SZ" "${tarfile}")

    cat <<EOD > ${PIGEN_FILESDIR}/release.json
{
    "version": "${version}",
    "releaseDate": "${releaseDate}",
    "channel": "${channel}",
    "platform": "linux-arm64"
}
EOD

    echo "pi-gen setup complete for simtezilo version ${version} (${channel} channel)"
}


function releaseImage() {
    date=$(date -u +"%Y-%m-%d")

    imageFile="${PIGEN_BASEDIR}/deploy/image_${date}-simtezilo-${version}-rpi-arm64-image.zip"
    releaseFile="./dist/releases/${channel}/${version}/simtezilo-${version}-rpi-arm64-image.zip"

    cp ${imageFile} ${releaseFile}
    sha256sum ${releaseFile} | cut -d' ' -f1 > ${releaseFile}.sha256
    
    echo "Release image created: ${releaseFile}"
}


case $release in
  "true") releaseImage;;
  "false") setupPiGen;;
  *) usage;;
esac
