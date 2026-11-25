package projections

import (
	"fmt"
	"goirc/events"

	"github.com/jmoiron/sqlx"
	"github.com/rcy/evoke"
)

type Projection struct {
	db *sqlx.DB
}

func New() (*Projection, error) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`create table channel_nicks(channel, nick)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`create table channels(channel)`)
	if err != nil {
		return nil, err
	}

	return &Projection{db: db}, nil
}

func (p *Projection) Subscribe(es *evoke.Service) {
	es.SubscribeSync(events.BotJoined{}, p.botJoined)
	es.SubscribeSync(events.NickJoined{}, p.userJoined)
}

func (p *Projection) botJoined(event evoke.Event, replay bool) error {
	fmt.Println("botJoined", event)

	_, err := p.db.Exec(`insert into channels(channel) values(?)`, event.AggregateID)
	if err != nil {
		return err
	}
	return nil
}

func (p *Projection) userJoined(event evoke.Event, replay bool) error {
	payload, err := evoke.UnmarshalPayload[events.NickJoined](event)
	if err != nil {
		return err
	}

	_, err = p.db.Exec(`insert into channel_nicks(channel,nick) values(?,?)`, event.AggregateID, payload.Nick)
	if err != nil {
		return err
	}

	return nil
}
