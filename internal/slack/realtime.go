package slack

import (
	"context"
	"fmt"
	"log"
	"os"

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
	debug        bool
}

type IncomingMessage struct {
	ChannelID string
	UserID    string
	Text      string
	Timestamp string
	ThreadTS  string
}

func NewRealtimeClient(slackClient *Client, appToken string, handler EventHandler, debug bool) *RealtimeClient {
	// Create a new Slack client with app token for socket mode
	opts := []slack.Option{
		slack.OptionAppLevelToken(appToken),
	}
	if debug {
		opts = append(opts, slack.OptionDebug(true))
		opts = append(opts, slack.OptionLog(log.New(os.Stderr, "slack: ", log.LstdFlags)))
	}
	appClient := slack.New("", opts...)

	socketOpts := []socketmode.Option{}
	if debug {
		socketOpts = append(socketOpts, socketmode.OptionDebug(true))
		socketOpts = append(socketOpts, socketmode.OptionLog(log.New(os.Stderr, "socketmode: ", log.LstdFlags)))
	}
	client := socketmode.New(appClient, socketOpts...)

	ctx, cancel := context.WithCancel(context.Background())

	return &RealtimeClient{
		client:       client,
		slackClient:  slackClient,
		eventHandler: handler,
		ctx:          ctx,
		cancel:       cancel,
		debug:        debug,
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
			if r.debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Received event type: %s\n", evt.Type)
			}
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					if r.debug {
						fmt.Fprintf(os.Stderr, "[DEBUG] Failed to cast to EventsAPIEvent\n")
					}
					continue
				}

				if r.debug {
					fmt.Fprintf(os.Stderr, "[DEBUG] EventsAPI inner event type: %s\n", eventsAPIEvent.InnerEvent.Type)
				}

				r.client.Ack(*evt.Request)

				switch innerEvent := eventsAPIEvent.InnerEvent.Data.(type) {
				case *slackevents.MessageEvent:
					if r.debug {
						fmt.Fprintf(os.Stderr, "[DEBUG] Message event: channel=%s user=%s text=%s\n",
							innerEvent.Channel, innerEvent.User, innerEvent.Text)
					}
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
