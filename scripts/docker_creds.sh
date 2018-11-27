# !/bin/bash
#
# Copyright (C) 2018 Nalej Group - All Rights Reserved
#
# Helper script to create docker credentials to access DockerHub images from private registry in DockerHub.
#
# Set environment variables with the credentials regarding your DockerHub account.
# export DOCKER_REGISTRY_SERVER=https://nalejregistry.azurecr.io
# export DOCKER_USER=Type the k8s service account user
# export DOCKER_PASSWORD=Type the k8s service account password
# export DOCKER_EMAIL=Type your email

kubectl --namespace=nalej create secret docker-registry nalej-registry \
  --docker-server=$DOCKER_REGISTRY_SERVER \
  --docker-username=$DOCKER_USER \
  --docker-password=$DOCKER_PASSWORD \
  --docker-email=$DOCKER_EMAIL
