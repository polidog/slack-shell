package slack

import (
	"context"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type EventHandler func(event interface{})

type RealtimeClient struct {
	client       *socketmode.Client
	slackClient  *Client
	eventHandler EventHandler
	ctx          context.Context
	cancel       context.CancelFunc
}

type IncomingMessage struct {
	ChannelID string
	UserID    string
	Text      string
	Timestamp string
	ThreadTS  string
}

func NewRealtimeClient(slackClient *Client, appToken string, handler EventHandler) *RealtimeClient {
	// Create a new Slack client with app token for socket mode
	appClient := slack.New(
		"", // User token not needed for socket mode connection
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(appClient)

	ctx, cancel := context.WithCancel(context.Background())

	return &RealtimeClient{
		client:       client,
		slackClient:  slackClient,
		eventHandler: handler,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (r *RealtimeClient) Start() error {
	go r.handleEvents()
	return r.client.Run()
}

func (r *RealtimeClient) Stop() {
	r.cancel()
}

func (r *RealtimeClient) handleEvents() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case evt := <-r.client.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}

				r.client.Ack(*evt.Request)

				switch innerEvent := eventsAPIEvent.InnerEvent.Data.(type) {
				case *slackevents.MessageEvent:
					msg := IncomingMessage{
						ChannelID: innerEvent.Channel,
						UserID:    innerEvent.User,
						Text:      innerEvent.Text,
						Timestamp: innerEvent.TimeStamp,
						ThreadTS:  innerEvent.ThreadTimeStamp,
					}
					if r.eventHandler != nil {
						r.eventHandler(msg)
					}
				}

			case socketmode.EventTypeConnectionError:
				if r.eventHandler != nil {
					r.eventHandler(evt.Data)
				}

			case socketmode.EventTypeConnected:
				if r.eventHandler != nil {
					r.eventHandler("connected")
				}

			case socketmode.EventTypeDisconnect:
				if r.eventHandler != nil {
					r.eventHandler("disconnected")
				}
			}
		}
	}
}
