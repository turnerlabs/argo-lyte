#!/bin/bash

set -e
set -x

wget https://github.com/turnerlabs/argo-lyte/blob/master/releases/0.0.2/argo-lyte-linux-amd64
chmod 700 /home/vagrant/argo-lyte-linux-amd64
sudo /home/vagrant/argo-lyte-linux-amd64 -workdirectory="${WORK_DIR}" -userurl="${USER_URL}"