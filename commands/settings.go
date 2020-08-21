package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func set(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	settings := database.GuildCache[m.GuildID]

	switch len(args) {
	case 0:
		showGuildSettings(s, m, settings)
	case 2:
		isAdmin, err := utils.MemberHasPermission(s, m.GuildID, m.Author.ID, discordgo.PermissionAdministrator)
		if err != nil {
			return err
		}
		if !isAdmin {
			return utils.ErrNoPermission
		}

		setting := args[0]
		newSetting := strings.ToLower(args[1])

		var passedSetting interface{}
		switch setting {
		case "pixiv":
			passedSetting, err = strconv.ParseBool(newSetting)
		case "prefix":
			if unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
				passedSetting = newSetting + " "
			} else {
				passedSetting = newSetting
			}

			if len(passedSetting.(string)) > 5 {
				return errors.New("new prefix is too long")
			}
		case "largeset":
			passedSetting, err = strconv.Atoi(newSetting)
		case "limit":
			passedSetting, err = strconv.Atoi(newSetting)
			if passedSetting.(int) == 0 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Why do you even have me here?")
				if err != nil {
					return err
				}
			}
		case "repost":
			if newSetting != "disabled" && newSetting != "enabled" && newSetting != "strict" {
				return errors.New("unknown option. repost only accepts enabled, disabled, and strict options")
			}

			passedSetting = newSetting
		case "reversesearch":
			if newSetting != "saucenao" && newSetting != "wait" {
				return errors.New("unknown option. reversesearch only accepts wait and saucenao options")
			}

			passedSetting = newSetting
		case "promptemoji":
			emoji, err := utils.GetEmoji(s, m.GuildID, newSetting)
			if err != nil {
				return errors.New("argument's either global emoji or not one at all")
			}
			passedSetting = emoji
		default:
			return errors.New("unknown setting " + setting)
		}

		if err != nil {
			return err
		}

		err = database.DB.ChangeSetting(m.GuildID, setting, passedSetting)
		if err != nil {
			return err
		}

		embed := &discordgo.MessageEmbed{
			Title: "✅ Successfully changed a setting!",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Setting",
					Value:  setting,
					Inline: true,
				},
				{
					Name:   "New value",
					Value:  newSetting,
					Inline: true,
				},
			},
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	default:
		return errors.New("incorrect command usage. Please use bt!help set command for more information")
	}

	return nil
}

func showGuildSettings(s *discordgo.Session, m *discordgo.MessageCreate, settings *database.GuildSettings) {
	guild, _ := s.Guild(settings.GuildID)

	emoji := ""
	if utils.EmojiRegex.MatchString(settings.PromptEmoji) {
		emoji = settings.PromptEmoji
	} else {
		emoji = "<:" + settings.PromptEmoji + ">"
	}
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Current settings",
		Description: guild.Name,
		Color:       utils.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Basic",
				Value: fmt.Sprintf("**Prefix:** %v", settings.Prefix),
			},
			{
				Name:  "Features",
				Value: fmt.Sprintf("**Pixiv:** %v\n**Reverse search:** %v\n**Repost:** %v", utils.FormatBool(settings.Pixiv), settings.ReverseSearch, settings.Repost),
			},
			{
				Name:  "Pixiv settings",
				Value: fmt.Sprintf("**Large set**: %v\n**Limit**: %v\n**Prompt emoji**: %v", settings.LargeSet, settings.Limit, emoji),
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: guild.IconURL(),
		},
		Timestamp: utils.EmbedTimestamp(),
	})
}
