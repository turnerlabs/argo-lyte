#!/bin/bash

set -e
set -x

wget -qO- https://github.com/turnerlabs/argo-lyte/releases/download/0.0.3/argo-lyte-linux-amd64 >> argo-lyte-linux-amd64
chmod 700 /home/vagrant/argo-lyte-linux-amd64
sudo /home/vagrant/argo-lyte-linux-amd64 -workdirectory="${WORK_DIR}" -userurl="${USER_URL}"