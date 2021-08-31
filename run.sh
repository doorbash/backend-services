#!/bin/bash

if [ "$1" == "stop" ]; then
    # sudo docker rm -f $(sudo docker ps -a -q)
    sudo docker-compose kill
    exit 0
fi

if [ "$1" == "clean" ]; then
    sudo rm -rf docker/db
    sudo mkdir -p docker/db
    sudo rm -rf docker/pg-admin/sessions
    sudo rm -rf docker/pg-admin/storage
    sudo rm -rf docker/pg-admin/pgadmin4.db*
    exit 0
fi

cd api
CGO_ENABLED=0 go build
if [ $? -ne 0 ]; then
    echo "Error while building api"
    exit 1
fi
cd ..

cd notification
CGO_ENABLED=0 go build
if [ $? -ne 0 ]; then
    echo "Error while building notifications"
    exit 1
fi
cd ..

chmod 777 docker/pg-admin/
chmod 777 docker/pg-admin/servers.json

sudo docker-compose up -d --force-recreate --build --remove-orphans

# shows api logs
if [ "$1" == "logs" ]; then
    sudo docker-compose logs -f $2
fi