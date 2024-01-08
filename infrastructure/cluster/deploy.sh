#!/bin/bash

set -o errexit

info() {
  echo "[INFO] $*"
}

fail() {
  echo "[ERROR] $*"
  exit 1
}

show_help() {
  cat <<EOF
Usage: $0 [OPTIONS]

Deploy a kind cluster with authentication for a private Docker registry.

Options:
  -c, --cluster-name NAME      Set the cluster name (default: "liferay-cluster").
  -f, --config-file PATH       Set the path to the Kind cluster configuration file.
  -u, --docker-username USER   Set the Docker registry username.
  -h, --help                   Show this help message.
EOF
}

docker_auth() {
  local -r username=$1
  local -r password=$2

  # Check required variables
  for var in username password; do
    if [ -z "${!var}" ]; then
      fail "Variable $var is not set"
    fi
  done

  (
    docker login -u "$username" -p "$password" 1>/dev/null ||
      fail "Failed to login to docker" &&
      info "Successfully logged in to docker with $username"
  )
}

create_kind_cluster() {
  local -r cluster_name=${1:-"liferay-cluster"}
  local -r config_file_path=$2

  # Private Registry Configuration
  local -r docker_config_path=$3
  export DOCKER_CONFIG_PATH="$docker_config_path"

  info "Deploying kind cluster $cluster_name from $config_file_path"
  local -r temp_config=$(mktemp)
  envsubst <"$config_file_path" >"$temp_config"
  cat "$temp_config"
  kind create cluster --name "$cluster_name" --config "$temp_config"
  info "Deployed kind cluster $cluster_name from $config_file_path"

  rm -rf "$temp_config"
}

deploy() {
  local -r cluster_name="liferay-cluster"
  local -r docker_output_path="$HOME/.docker/config.json"

  local config_file_path=""
  local docker_username=""
  local docker_password="$DOCKER_PASSWORD"

  while [[ $# -gt 0 ]]; do
    case "$1" in
    -c | --cluster-name)
      cluster_name="$2"
      shift
      shift
      ;;
    -f | --config-file)
      config_file_path="$2"
      shift
      shift
      ;;
    -u | --docker-username)
      docker_username="$2"
      shift
      shift
      ;;
    -h | --help)
      show_help
      exit 0
      ;;
    *)
      fail "Unknown option: $1"
      ;;
    esac
  done

  if [ -z "$docker_password" ]; then
    read -r -s -p "Enter your Docker registry password: " docker_password
  else
    info "Using Docker registry password from environment variable"
  fi

  if [ -z "$config_file_path" ] || [ -z "$docker_username" ] || [ -z "$docker_password" ]; then
    fail "Missing required options. Use --help for usage information."
  fi

  docker_auth "$docker_username" "$docker_password"
  create_kind_cluster "$cluster_name" "$config_file_path" "$docker_output_path"
}

deploy "$@"
