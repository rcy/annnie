package bot

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"goirc/bot/idle"
	"goirc/commit"
	"goirc/db/model"
	"goirc/events"
	db "goirc/model"
	"goirc/model/laters"
	"goirc/util"
	"log"
	"log/slog"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/rcy/evoke"
	irc "github.com/thoj/go-ircevent"
)

type Handler struct {
	pattern string
	regexp  regexp.Regexp
	action  HandlerFunction
}

func NewHandler(pattern string, regexp regexp.Regexp, action HandlerFunction) *Handler {
	return &Handler{pattern, regexp, action}
}

func (h Handler) Regexp() *regexp.Regexp {
	return &h.regexp
}

func (h Handler) String() string {
	strs := strings.Split(runtime.FuncForPC(reflect.ValueOf(h.action).Pointer()).Name(), ".")
	return fmt.Sprintf("%-32s %s", h.pattern, strs[len(strs)-1])
}

type delivery struct {
	target  string
	message string
}

type Bot struct {
	Conn               *irc.Connection
	Channel            string
	Handlers           []Handler
	LastEvent          *irc.Event
	IsJoined           bool
	idleResetFunctions []func()
	queue              chan delivery
	Events             evoke.Inserter
}

func (b *Bot) Handle(pattern string, action HandlerFunction) {
	h := Handler{
		pattern,
		*regexp.MustCompile(pattern),
		action,
	}

	b.Handlers = append(b.Handlers, h)
}

func (b *Bot) Repeat(timeout time.Duration, action HandlerFunction) {
	go func() {
		for {
			time.Sleep(timeout)
			err := action(HandlerParams{
				privmsgf: b.MakePrivmsgf(),
				target:   b.Channel,
			})
			if err != nil {
				slog.Warn("Repeat", "err", err)
			}
		}
	}()
}

func (b *Bot) IdleRepeat(timeout time.Duration, action HandlerFunction) {
	reset := idle.Repeat(timeout, func() {
		err := action(HandlerParams{
			privmsgf: b.MakePrivmsgf(),
			target:   b.Channel,
		})
		if err != nil {
			slog.Warn("IdleRepeat", "err", err)
		}
	})

	b.idleResetFunctions = append(b.idleResetFunctions, reset)
}

func (b *Bot) IdleRepeatAfterReset(timeout time.Duration, action HandlerFunction) {
	reset := idle.RepeatAfterReset(timeout, func() {
		err := action(HandlerParams{
			privmsgf: b.MakePrivmsgf(),
			target:   b.Channel,
		})
		if err != nil {
			slog.Warn("IdleRepeatAfterReset", "err", err)
		}
	})

	b.idleResetFunctions = append(b.idleResetFunctions, reset)
}

func (b *Bot) resetIdle() {
	for _, fn := range b.idleResetFunctions {
		fn()
	}
}

func (b *Bot) Loop() {
	b.Conn.Loop()
}

func Connect(es *evoke.Service, nick string, channel string, server string, saslLogin string, saslPassword string) (*Bot, error) {
	initialized := make(chan bool)
	var bot Bot

	bot.Events = es
	bot.Channel = channel
	bot.Conn = irc.IRC(nick, "github.com/rcy/annnie")
	bot.Conn.VerboseCallbackHandler = false
	bot.Conn.Debug = true
	bot.Conn.UseTLS = true
	bot.Conn.UseSASL = true
	bot.Conn.SASLLogin = saslLogin
	bot.Conn.SASLPassword = saslPassword
	bot.Conn.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	bot.Conn.AddCallback("001", func(e *irc.Event) {
		bot.Conn.Join(channel)
	})
	bot.Conn.AddCallback("353", func(e *irc.Event) {
		// clear the presence of all channel nicks
		_, err := db.DB.Exec(`update channel_nicks set updated_at = current_timestamp, present = false where present = true`)
		if err != nil {
			log.Fatal(err)
		}

		// remove @ op markers from nick argument
		nickStr := strings.ReplaceAll(e.Arguments[len(e.Arguments)-1], "@", "")

		nicks := strings.Split(nickStr, " ")

		es.MustInsert(bot.Channel, events.NamesListed{Nicks: nicks})

		// mark nicks as present and record timestamp which can be intepreted as 'last seen', or 'online since'
		for _, nick := range nicks {
			_, err = db.DB.Exec(`
insert into channel_nicks(updated_at, channel, nick, present) values(current_timestamp, ?, ?, ?)
on conflict(channel, nick) do update set updated_at = current_timestamp, present=excluded.present`,
				channel, nick, true)
			if err != nil {
				log.Fatal(err)
			}
		}
	})
	bot.Conn.AddCallback("366", func(e *irc.Event) {})
	bot.Conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Arguments[0] == bot.Channel {
			es.MustInsert(bot.Channel, events.MessageReceived{Nick: e.Nick, Content: e.Arguments[1]})
			bot.resetIdle()
		} else {
			es.MustInsert(e.Nick, events.PrivateMessageReceived{Content: e.Arguments[1]})
		}
		go bot.RunHandlers(e)
	})
	bot.Conn.AddCallback("JOIN", func(e *irc.Event) {
		fmt.Println("JOIN", e.Nick)
		if e.Nick != bot.Conn.GetNick() {
			es.MustInsert(bot.Channel, events.NickJoined{Nick: e.Nick})

			go func() {
				time.Sleep(10 * time.Second)
				bot.SendLaters(channel, e.Nick)
			}()

			go func() {
				err := bot.SendMissed(context.TODO(), channel, e.Nick)
				if err != nil {
					panic(err)
				}
			}()

			// trigger NAMES to update the list of joined nicks
			bot.Conn.SendRawf("NAMES %s", channel)
		} else {
			es.MustInsert(bot.Channel, events.BotJoined{Nick: e.Nick})

			go func() {
				bot.IsJoined = true
				time.Sleep(1 * time.Second)
				url, err := commit.URL()
				if err != nil {
					bot.Conn.Privmsgf(channel, "error: %s", err)
					return
				}
				if url != "" {
					bot.Conn.Privmsgf(channel, "%s", url)
				}
				initialized <- true
			}()
		}
	})
	bot.Conn.AddCallback("PART", func(e *irc.Event) {
		if e.Nick != nick {
			es.MustInsert(bot.Channel, events.NickParted{Nick: e.Nick})

			// trigger NAMES to update the list of joined nicks
			bot.Conn.SendRawf("NAMES %s", channel)
		} else {
			es.MustInsert(bot.Channel, events.BotParted{Nick: e.Nick})
			bot.IsJoined = false
		}
	})
	bot.Conn.AddCallback("QUIT", func(e *irc.Event) {
		if e.Nick != nick {
			es.MustInsert(bot.Channel, events.NickQuit{Nick: e.Nick})

			// trigger NAMES to update the list of joined nicks
			bot.Conn.SendRawf("NAMES %s", channel)
		} else {
			es.MustInsert(bot.Channel, events.BotQuit{Nick: e.Nick})
			bot.IsJoined = false
		}
	})
	bot.Conn.AddCallback("NICK", func(e *irc.Event) {
		if e.Nick != nick {
			// trigger NAMES to update the list of joined nicks
			bot.Conn.SendRawf("NAMES %s", channel)
		}
	})

	bot.setupDeliveryQueue()

	err := bot.Conn.Connect(server)

	<-initialized

	return &bot, err
}

func (bot *Bot) setupDeliveryQueue() {
	const (
		queueSize       = 100
		initialDelay    = 100 * time.Millisecond // starting delay
		delayMultiplier = 1.5                    // how much the delay grows each time
		maxDelay        = time.Second            // maximum delay
		coolOffDuration = 10 * time.Second       // how long before we reset
	)

	bot.queue = make(chan delivery, queueSize)

	go func() {
		delay := initialDelay
		lastSendTime := time.Now()
		for d := range bot.queue {
			if time.Now().Sub(lastSendTime) > coolOffDuration {
				delay = initialDelay
			}
			bot.Conn.Privmsg(d.target, d.message)
			lastSendTime = time.Now()

			// log sent event
			if d.target == bot.Channel {
				_ = bot.Events.Insert(d.target, events.MessageSent{Content: d.message})
			} else {
				_ = bot.Events.Insert(d.target, events.PrivateMessageSent{Content: d.message})
			}

			time.Sleep(delay)

			delay = time.Duration(float64(delay) * delayMultiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}()
}

func (bot *Bot) SendLaters(channel string, nick string) {
	// loop through each later message and see if the prefix matches this nick
	rows, err := laters.Get()
	if err != nil {
		log.Fatal(err)
	}
	for _, row := range rows {
		if strings.Contains(nick, row.Target) {
			_, err := db.DB.Exec(`delete from laters where rowid = ?`, row.RowId)
			if err != nil {
				log.Fatal(err)
			}
			bot.Conn.Privmsgf(channel, "%s: %s (from %s %s ago)", nick, row.Message, row.Nick, util.Since(row.CreatedAt))
		}
	}
}

func (bot *Bot) MakePrivmsgf() func(string, string, ...interface{}) {
	return func(target, message string, a ...interface{}) {
		str := fmt.Sprintf(message, a...)

		lines := strings.Split(str, "\n")

		for _, line := range lines {
			chunks := splitString(line, 420)
			for _, chunk := range chunks {
				bot.queue <- delivery{target: target, message: chunk}
			}
		}
	}
}

func splitString(data string, chunkSize int) []string {
	var chunks []string

	for len(data) > 0 {
		if len(data) < chunkSize {
			chunkSize = len(data)
		}
		chunks = append(chunks, data[:chunkSize])
		data = data[chunkSize:]
	}

	return chunks
}

func (bot *Bot) RunHandlers(e *irc.Event) {
	channel := e.Arguments[0]
	msg := e.Arguments[1]
	nick := e.Nick

	var target string
	if channel == bot.Conn.GetNick() {
		target = nick
	} else {
		target = channel
	}

	for _, handler := range bot.Handlers {
		matches := handler.regexp.FindStringSubmatch(msg)
		if len(matches) > 0 {
			err := handler.action(HandlerParams{
				privmsgf:  bot.MakePrivmsgf(),
				msg:       msg,
				nick:      nick,
				target:    target,
				matches:   matches,
				LastEvent: bot.LastEvent,
			})

			if err != nil {
				msg := err.Error()
				if len(msg) > 1024 {
					msg = msg[:1024] // prevent spamming the channel
				}
				bot.MakePrivmsgf()(target, "error: %s", msg)
				return
			}
		}
	}
	bot.LastEvent = e
}

func isAltNick(nick string) bool {
	return strings.HasSuffix(nick, "`") || strings.HasSuffix(nick, "_")
}

func (bot *Bot) SendMissed(ctx context.Context, channel string, nick string) error {
	q := model.New(db.DB)

	if isAltNick(nick) {
		return nil
	}

	channelNick, err := q.ChannelNick(ctx, model.ChannelNickParams{Nick: nick, Channel: channel, Present: false})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// first time seeing this nick
			return nil
		}
		return fmt.Errorf("ChannelNick: %w", err)
	}

	notes, err := q.ChannelNotesSince(ctx, model.ChannelNotesSinceParams{Target: channel, CreatedAt: channelNick.UpdatedAt})
	if err != nil {
		return fmt.Errorf("ChannelNotesSince: %w", err)
	}

	if len(notes) > 0 {
		bot.Conn.Privmsgf(nick, "Hi %s, you missed %d thing(s) in %s since %s:",
			nick, len(notes), channel, channelNick.UpdatedAt)

		for _, note := range notes {
			text := note.Text.String
			meta := util.Ago(time.Since(note.CreatedAt).Round(time.Second)) + " ago"

			if note.Anon {
				if note.Kind == "link" {
					var err error
					text, err = note.Link()
					if err != nil {
						return err
					}
				}
			} else {
				meta = "from " + note.Nick.String + " " + meta
			}

			bot.Conn.Privmsgf(nick, "%s (%s)", text, meta)
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}
