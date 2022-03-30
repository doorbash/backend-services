package util

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"

	"google.golang.org/api/option"
)

func SendNotification(
	ctx context.Context,
	projectId string,
	topic string,
	data map[string]string,
) error {
	opt := option.WithCredentialsFile(fmt.Sprintf("/fcm/%s.json", projectId))
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("error initializing app: %v", err)
	}
	c, err := app.Messaging(ctx)
	if err != nil {
		return err
	}
	_, err = c.Send(ctx, &messaging.Message{
		Data:  data,
		Topic: topic,
	})
	if err != nil {
		return err
	}
	return nil
}
