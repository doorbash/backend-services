module github.com/doorbash/backend-services/notification

go 1.16

require (
	github.com/doorbash/backend-services/api v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v4 v4.13.0
)

replace github.com/doorbash/backend-services/api => ../api
