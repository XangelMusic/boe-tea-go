package commands

import (
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

var (
	//Router ...
	Router *gumi.Gumi
)

func init() {
	Router = gumi.NewGumi(gumi.WithErrorHandler(func(e error) *discordgo.MessageSend {
		if e != nil {
			eb := embeds.NewBuilder()
			embed := eb.ErrorTemplate(e.Error()).Finalize()

			return &discordgo.MessageSend{
				Embed: embed,
			}
		}
		return nil
	}), gumi.WithPrefixResolver(func(g *gumi.Gumi, s *discordgo.Session, m *discordgo.MessageCreate) []string {
		if guild, ok := database.GuildCache[m.GuildID]; ok {
			if guild.Prefix == "bt!" {
				return []string{"bt!", "bt ", "bt.", "<@!" + s.State.User.ID + ">"}
			}
			return []string{guild.Prefix, "<@!" + s.State.User.ID + ">"}
		}
		return []string{"bt!", "bt ", "bt.", "<@!" + s.State.User.ID + ">"}
	}))

	generalGroup := Router.Groups["general"]
	generalGroup.AddCommand(&gumi.Command{
		Name:        "ping",
		Description: "Checks if Boe Tea is online",
		Exec:        ping,
	})

	feedbackHelp := gumi.NewHelpSettings()
	feedbackHelp.AddField("Usage", "``bt!feedback [feedback message]``. Please use this command to report bugs or suggest new features only. If you misuse this command you'll get blacklisted!", false)
	feedbackHelp.AddField("feedback message", "While suggestions can be plain text, bug reports are expected to be formatted in a specific way. Template shown below:\n```**Summary:** -\n**Reproduction:** -\n**Expected result:** -\n**Actual result:** -```\nYou can provide images as links or a single image as an attachment to the feedback message!", false)

	generalGroup.AddCommand(&gumi.Command{
		Name:        "feedback",
		Description: "Reach out to bot's author! Use ``bt!help feedback`` to get a template",
		Exec:        feedback,
		Help:        feedbackHelp,
	})

	generalGroup.AddCommand(&gumi.Command{
		Name:        "about",
		Aliases:     []string{"support", "invite"},
		Description: "Boe Tea's about page",
		Exec:        about,
	})

	setHelp := gumi.NewHelpSettings()
	setHelp.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!set ``<setting>`` ``<new setting>``",
		},
		{
			Name:  "prefix",
			Value: "Bot's prefix. Up to ***5 characters***. If last character is a letter whitespace is assumed (takes one character).",
		},
		{
			Name:  "largeset",
			Value: "Album size considered as large and invokes a prompt when posted.",
		},
		{
			Name:  "limit",
			Value: "Hard limit for album size. Only first image from an album will be posted if album size exceeded limit.",
		},
		{
			Name:  "pixiv | twitter",
			Value: "Pixiv or Twitter reposting switch, valid parameters: ***[enabled, on, t, true], [disabled, off, f, false]***",
		},
		{
			Name:  "repost",
			Value: "Repost check setting, valid parameters: ***[enabled, disabled, strict]***. Strict mode disables a prompt and removes reposts on sight.",
		},
		{
			Name:  "reversesearch",
			Value: "Default reverse image search engine. Available options: ***[saucenao, wait]***",
		},
		{
			Name:  "promptemoji",
			Value: "Confirmation prompt emoji. Only unicode or local server emoji's are allowed.",
		},
	}

	generalGroup.AddCommand(&gumi.Command{
		Name:        "set",
		Aliases:     []string{"config", "cfg", "settings"},
		Description: "Show or change server's settings",
		Help:        setHelp,
		Exec:        set,
		GuildOnly:   true,
		Cooldown:    5 * time.Second,
	})
}