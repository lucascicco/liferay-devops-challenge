#!/bin/bash

readonly -a HELM_REPOS=(
  "bitnami https://charts.bitnami.com/bitnami"
)

add_helm_repos_and_update() {
  echo "[INFO] Adding Helm repositories and updating..."
  for repo in "${HELM_REPOS[@]}"; do
    helm repo add $repo
  done
  helm repo update
  echo "[INFO] Helm repositories added and updated."
}

add_helm_repos_and_update

exit 0
