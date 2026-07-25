package service

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct{
	session    *discordgo.Session
	channelIds map[string]string
	service *TournamentService
	lastPublish time.Time
	published []int
	timers []*time.Timer
}

const channelName = "tournaments"
var location *time.Location

func InitBot(token string, service *TournamentService) (*Bot, error) {
	l, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, err
	}
	location = l

	channelIds := map[string]string{}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Bot startup failed %s", err)
	}
	dg.ShouldReconnectOnError = true

	dg.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) {
		log.Printf("Bot is ready")
	})

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.GuildCreate) {
		log.Printf("Guild \"%s\" added", r.Guild.Name)
		channels := r.Guild.Channels
		for _, channel := range channels {
			if channel.Name == channelName {
				channelIds[channel.GuildID] = channel.ID
				log.Printf("Channel \"%s\" added for \"%s %s\"", channelName, r.Guild.Name, channel.ID)
			}
		}
	})

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.GuildDelete) {
		log.Printf("guild %s removed", r.Guild.Name)
		delete(channelIds, r.Guild.ID)
	})

	err = dg.Open()
	if err != nil {
		return nil, err
	}

	bot := &Bot{service: service, lastPublish: time.Now(), published: []int{}, session: dg, channelIds: channelIds}
	bot.timers = []*time.Timer{
		scheduleDaily(9, 0, bot.PublishTurnamentRegistrations),
		scheduleDaily(14, 0, bot.PublishTurnamentRegistrations),
		scheduleDaily(18, 0, bot.PublishTurnamentRegistrations),
		scheduleDaily(19, 30, bot.PublishTurnamentRegistrations),
		scheduleDaily(21, 00, bot.PublishTurnamentRegistrations),
		scheduleDaily(7, 00, bot.PublishTurnamentRegistrations),
	}
	return bot, nil
}

func (b *Bot) sendMessage(m string) {
	for _, channelId := range b.channelIds {
		log.Printf("SEND MESSAGE TO %s\n", channelId)
		_, err := b.session.ChannelMessageSend(channelId, m)
		if err != nil {
			log.Printf("Error while sending message to %s", channelId)
			return
		}
	}
}

func (b *Bot) Close() {
	b.session.Close()
	for _, t := range b.timers {
		t.Stop()
	}
}

func (b *Bot)PublishTurnamentRegistrations() {
	now := time.Now()
	log.Printf("Publish scheduled\n")

	if b.lastPublish.Day() != now.Day() {
		b.published = []int{}
	}


	for _, t := range b.service.GetTournaments() {
		if slices.Contains(b.published, t.Id) {
			continue
		}
		for _, r := range t.Registrations {
			d := r.StartDate.Sub(now)
			if d > 0 && d < time.Duration(now.Hour()*int(time.Hour)+now.Minute()*int(time.Minute)+now.Second()*int(time.Second)) {
				day := "heute"
				if r.StartDate.Day() > now.Day() {
					day = "morgen"
				}
				text := "⏰ Turnieranmeldung für \"**%s**\"\n%s\nöffnet **%s um %s Uhr**\n📍 %s\n🔗 https://turniere.discgolf.de/index.php?p=events&sp=view&id=%d\n"
				b.sendMessage(fmt.Sprintf(text, t.Title, r.Title, day, r.StartDate.Format("15:04"), t.Localtion, t.Id))
				b.published = append(b.published, t.Id)
				log.Printf("Published %v\n", t.Id)
			}
		}
	}

	b.lastPublish = now
}

func scheduleDaily(hour, min int, task func()) *time.Timer {
    var next func() time.Time
    next = func() time.Time {
        now := time.Now()
        n := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, location)
        if !n.After(now) {
            n = n.Add(24 * time.Hour)
        }
        return n
    }

    var timer *time.Timer
    var run func()
    run = func() {
        task()
        timer.Reset(time.Until(next()))
    }
    timer = time.AfterFunc(time.Until(next()), run)
    return timer
}
