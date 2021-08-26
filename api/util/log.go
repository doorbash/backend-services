package util

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4"
)

type DatabaseLogger struct{}

func (d *DatabaseLogger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	sql, ok := data["sql"].(string)
	if !ok {
		log.Println("DatabaseLogger:", msg, data["sql"])
	} else {
		query, err := SanitizeSQL(sql, (data["args"].([]interface{}))...)
		if err != nil {
			return
		}
		log.Println("DatabaseLogger:", msg, query)
		if data["err"] != nil {
			log.Println("DatabaseLogger:", "Error:", data["err"])
		}
	}
}
