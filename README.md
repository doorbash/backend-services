# backend-services-api
[UNDER DEVELOPMENT] A couple of backend service APIs for Android apps (remote config, push, ads) 

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
API_PATH="/api"
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
IMAGE_NAME_PGADMIN="dpage/pgadmin4:5.6"
IMAGE_NAME_API="backend-ir/api"
IMAGE_NAME_NGINX="nginx:1.21.1-alpine"
```

## Run
```
./run.sh logs
```

## Postman
https://documenter.getpostman.com/view/13117984/TzzGGtSs

## Todo
- Android client
- Push notification (FCM, Pull based)
- Ads
- Web panel (help needed!)