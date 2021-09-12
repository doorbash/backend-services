# backend-services
A couple of simple backend service APIs for Android apps:
- Remote config
- Pull notification 

## Prerequisites
- Go
- Docker

## Configuration
- Put SSL key files in:
```
docker/nginx/cert/fullchain.pem
docker/nginx/cert/privkey.pem
```

- Create `docker/pg-admin/servers.json`:
```
{
    "Servers": {
        "1": {
            "Name": "backend-services",
            "Group": "Servers",
            "Port": 5432,
            "Username": "PUT_DATABASE_USER_HERE",
            "Host": "db",
            "SSLMode": "disable",
            "MaintenanceDB": "postgres"
        }
    }
}
```

- Create `.env`:
```
API_MODE="private"
API_LISTEN_ADDR=":8080"
API_ADMIN_EMAIL="PUT_YOUR_EMAIL_ADDRESS_HERE"

DATABASE_USER="PUT_DATABASE_USER_HERE"
DATABASE_PASSWORD="PUT_DATABASE_PASSWORD_HERE"
DATABASE_NAME="api"

AUTH_CLIENT_ID="PUT_OAUTH2_CLIENT_ID_HERE"
AUTH_CLIENT_SECRET="PUT_OAUTH2_CLIENT_SECRET_HERE"
AUTH_SESSION_KEY="PUT_A_RANDOM_LONG_STRING_HERE"

PGADMIN_DEFAULT_EMAIL="PUT_PG_ADMIN_EMAIL_HERE"
PGADMIN_DEFAULT_PASSWORD="PUT_PG_ADMIN_PASSWORD_HERE"

IMAGE_NAME_POSTGRES="postgres:14beta3-alpine3.14"
IMAGE_NAME_PGADMIN="dpage/pgadmin4:5.7"
IMAGE_NAME_NGINX="nginx:1.21.1-alpine"
IMAGE_NAME_REDIS="redis:6.2.5-alpine3.14"
IMAGE_NAME_API="backend-services/api"
IMAGE_NAME_NOTIFICATION="backend-services/notification"
```

## Run
```
./run.sh logs api
```

## Client
https://github.com/doorbash/backend-services-android

## Postman
https://documenter.getpostman.com/view/13117984/TzzGGtSs

## Todo
- [x] Android client
- [ ] Ads
- [ ] Web panel