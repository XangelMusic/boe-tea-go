package bot

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/commands"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	bannedUsers = ttlcache.NewCache()
	BoeTea      *Bot
)

type Bot struct {
	Session *discordgo.Session
}

func init() {
	//bannedUsers cache makes sure banned users don't have their favourites removed
	bannedUsers.SetTTL(15 * time.Second)
}

func (b *Bot) Run() error {
	if err := b.Session.Open(); err != nil {
		return err
	}

	defer b.Session.Close()
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGSEGV, syscall.SIGHUP)
	<-sc

	return nil
}

func NewBot(token string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	bot := &Bot{dg}
	dg.AddHandler(bot.messageCreated)
	dg.AddHandler(bot.onReady)
	dg.AddHandler(bot.reactCreated)
	dg.AddHandler(bot.guildCreated)
	dg.AddHandler(bot.guildDeleted)
	dg.AddHandler(bot.reactRemoved)
	dg.AddHandler(bot.guildBanAdd)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	BoeTea = bot
	return bot, nil
}

func (b *Bot) onReady(_ *discordgo.Session, e *discordgo.Ready) {
	log.Infoln(e.User.String(), "is ready.")
	log.Infof("Connected to %v guilds!", len(e.Guilds))
}

func (b *Bot) prefixless(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var (
		art = repost.NewPost(m)
	)

	err := art.Post(s)
	if err != nil {
		log.Warnln("art.Post():", err)
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		err := art.Crosspost(s, channels)
		if err != nil {
			log.Warnln("art.Crosspost():", err)
		}
	}

	return nil
}

func (b *Bot) messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}
	isCommand := commands.Router.Handle(s, m)
	if !isCommand && m.GuildID != "" {
		err := b.prefixless(s, m)
		commands.Router.ErrorHandler(err)
	}
}

func (b *Bot) reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}

	addFavourite := func(nsfw bool) {
		user := database.DB.FindUser(r.UserID)
		if user == nil {
			log.Infof("User not found. Adding a new user. User ID: %v", r.UserID)
			user = database.NewUserSettings(r.UserID)
			err := database.DB.InsertOneUser(user)
			if err != nil {
				log.Warnf("User while adding a user. User ID: %v. Err: %v", r.UserID, err)
				return
			}
		}

		if msg, err := s.ChannelMessage(r.ChannelID, r.MessageID); err != nil {
			log.Warnf("reactCreated() -> s.ChannelMessage(): %v", err)
		} else {
			if len(msg.Embeds) != 0 && msg.Author.ID == s.State.User.ID {
				if msg.Embeds[0].URL != "" {
					msg.Content = msg.Embeds[0].URL
				}
			}
			art := repost.NewPost(&discordgo.MessageCreate{Message: msg})
			if art.Len() == 0 {
				return
			}

			var artwork *database.Artwork
			switch {
			case len(art.PixivMatches) > 0:
				pixivID := ""
				for k := range art.PixivMatches {
					pixivID = k
					break
				}

				log.Infof("Detected Pixiv art to favourite. User ID: %v. Pixiv ID: %v", r.UserID, pixivID)
				pixiv, err := ugoira.PixivApp.GetPixivPost(pixivID)
				if err != nil {
					log.Warnf("addFavorite -> GetPixivPost: %v", err)
					return
				}

				artwork = &database.Artwork{
					Title:     pixiv.Title,
					URL:       pixiv.URL,
					Author:    pixiv.Author,
					Images:    pixiv.Images.ToArray(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
			case len(art.TwitterMatches) > 0:
				twitterURL := ""
				for k := range art.TwitterMatches {
					twitterURL = "https://twitter.com/i/status/" + k
					break
				}

				log.Infof("Detected Twitter art to favourite. User ID: %v. Tweet: %v", r.UserID, twitterURL)
				tweet, err := tsuita.GetTweet(twitterURL)
				if err != nil {
					log.Warnf("addFavorite -> GetTwitterPost: %v", err)
					return
				}

				if len(tweet.Gallery) > 0 {
					artwork = &database.Artwork{
						Author:    tweet.Username,
						Images:    tweet.Images(),
						URL:       tweet.URL,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}
				}
			}

			if artwork != nil {
				artwork, err := database.DB.AddFavourite(r.UserID, artwork, nsfw)
				if err != nil {
					log.Warnf("database.DB.AddFavourite() -> Error while adding a favourite: %v", err)
				} else if user.DM {
					ch, err := s.UserChannelCreate(user.ID)
					if err != nil {
						log.Warnf("s.UserChannelCreate -> %v", err)
					} else {
						var (
							eb          = embeds.NewBuilder()
							description = fmt.Sprintf("Don't like DMs? Execute `bt!userset dm disabled`\n```\nID: %v\nURL: %v\nNSFW: %v```", artwork.ID, artwork.URL, nsfw)
						)
						eb.Title("✅ Sucessfully added an artwork to favourites").Description(description)
						if len(artwork.Images) > 0 {
							eb.Thumbnail(artwork.Images[0])
						}

						s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
					}
				}
			}
		}
	}

	switch r.Emoji.APIName() {
	case "❌":
		if repost.MsgCache.Count() > 0 {
			key := r.ChannelID + r.MessageID
			cache, ok := repost.MsgCache.Get(key)
			if ok {
				cache := cache.(*repost.CachedMessage)
				if cache != nil {
					if cache.OriginalMessage.Author.ID != r.UserID {
						return
					}
					err := s.ChannelMessageDelete(cache.SentMessage.ChannelID, cache.SentMessage.ID)
					if err != nil {
						log.Warnf("ChannelMessageDelete(): %v", err)
					}
				}
			}
		}
	case "💖":
		addFavourite(false)
	case "🤤":
		addFavourite(true)
	}
}

func (b *Bot) reactRemoved(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}

	if _, f := bannedUsers.Get(r.UserID); f {
		return
	}

	if r.Emoji.APIName() == "💖" || r.Emoji.APIName() == "🤤" {
		user := database.DB.FindUser(r.UserID)
		if user != nil {
			if msg, err := s.ChannelMessage(r.ChannelID, r.MessageID); err != nil {
				log.Warnf("reactCreated() -> s.ChannelMessage(): %v", err)
			} else {
				if len(msg.Embeds) != 0 && msg.Author.ID == s.State.User.ID {
					if msg.Embeds[0].URL != "" {
						msg.Content = msg.Embeds[0].URL
					}
				}
				art := repost.NewPost(&discordgo.MessageCreate{Message: msg})
				if art.Len() == 0 {
					return
				}

				switch {
				case len(art.PixivMatches) > 0:
					log.Infof("Removing a favourite. User ID: %v", r.UserID)

					pixivURL := ""
					for k := range art.PixivMatches {
						pixivURL = "https://pixiv.net/en/artworks/" + k
						break
					}

					artwork, err := database.DB.RemoveFavouriteURL(user.ID, pixivURL)
					if err != nil {
						log.Warnln("DeleteFavouriteURL -> %v", err)
					} else if user.DM {
						ch, err := s.UserChannelCreate(user.ID)
						if err != nil {
							log.Warnf("s.UserChannelCreate -> %v", err)
						} else {
							eb := embeds.NewBuilder()
							eb.Title("✅ Sucessfully removed an artwork from favourites")
							eb.Description(fmt.Sprintf("```\nURL: %v```", pixivURL))
							eb.Thumbnail(artwork.Images[0])
							s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
						}
					}
				case len(art.TwitterMatches) > 0:
					log.Infof("Removing a favourite. User ID: %v", r.UserID)
					twitterURL := ""
					for k := range art.TwitterMatches {
						twitterURL = "https://twitter.com/i/status/" + k
						break
					}

					tweet, err := tsuita.GetTweet(twitterURL)
					if err != nil {
						log.Warnf("reactRemoved -> GetTweet: %v", err)
						return
					}

					artwork, err := database.DB.RemoveFavouriteURL(user.ID, tweet.URL)
					if err != nil {
						log.Warnln("DeleteFavouriteURL -> %v", err)
					} else if user.DM {
						ch, err := s.UserChannelCreate(user.ID)
						if err != nil {
							log.Warnf("s.UserChannelCreate -> %v", err)
						} else {
							eb := embeds.NewBuilder()
							eb.Title("✅ Sucessfully removed an artwork from favourites")
							eb.Thumbnail(artwork.Images[0])
							eb.Description(fmt.Sprintf("Don't like DMs? Execute `bt!userset dm disabled`\n```\nURL: %v```", twitterURL))

							s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
						}
					}
				}
			}
		}
	}
}

func (b *Bot) guildCreated(_ *discordgo.Session, g *discordgo.GuildCreate) {
	if _, ok := database.GuildCache[g.ID]; !ok {
		newGuild := database.DefaultGuildSettings(g.ID)
		err := database.DB.InsertOneGuild(newGuild)
		if err != nil {
			log.Warnln(err)
		}

		database.GuildCache[g.ID] = newGuild
		log.Infoln("Joined", g.Name)
	}
}

func (b *Bot) guildDeleted(_ *discordgo.Session, g *discordgo.GuildDelete) {
	if g.Unavailable {
		log.Infoln("Guild outage. ID: ", g.ID)
	} else {
		log.Infoln("Kicked/banned from a guild. ID: ", g.ID)
	}
}

func (b *Bot) guildBanAdd(_ *discordgo.Session, m *discordgo.GuildBanAdd) {
	bannedUsers.Set(m.User.ID, m.GuildID)
}
