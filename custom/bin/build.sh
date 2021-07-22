#!/usr/bin/env bash

set -e
set -x

RELEASES_DIR=/home/git/releases
GITEA_REPO=https://github.com/unfoldingWord-dev/dcs.git

version=$1 # THIS NEEDS TO BE THE VERSION WE ARE MAKING WITHOUT the "v", e.g. 1.0.0

# MAKE A TEMP go DIRECTORY
cd $(mktemp -d ~/tmp/go-XXXX)

# SET GO PATHS FOR COMPILING
export GOPATH=$(pwd)
export PATH=/usr/local/go/bin:$GOPATH/bin:$PATH

# COMPILE GITEA FROM OUR GITEA_REPO
go get -d -u code.gitea.io/gitea
cd src/code.gitea.io
rm -rf gitea
git clone --branch master ${GITEA_REPO} gitea
cd gitea
TAGS="bindata" make generate build

# SET GITEA PATH
export GITEA_PATH=${GOPATH}/src/code.gitea.io/gitea

# MAKE THE RELEASE DIR
rm -rf ${RELEASES_DIR}/${version}
RELEASE_PATH=${RELEASES_DIR}/${version}/gitea
mkdir -p ${RELEASE_PATH}

# COPY IN gitea and make custom dir from $CUSTOM_REPO
cp ${GITEA_PATH}/gitea ${RELEASE_PATH}
cp -r ${GITEA_PATH}/custom ${RELEASE_PATH}

# TAR IT UP
tar -cvzf ${RELEASES_DIR}/linux_amd64_${version}.tar.gz -C ${RELEASES_DIR}/${version} gitea

