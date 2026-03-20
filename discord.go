package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

const verifyCommandName = "verify"

type discordBot struct {
	session       *discordgo.Session
	cfg           *Config
	authenticator *googleAuthenticator
}

func newDiscordBot(cfg *Config, authenticator *googleAuthenticator) (*discordBot, error) {
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, err
	}

	bot := &discordBot{
		session:       session,
		cfg:           cfg,
		authenticator: authenticator,
	}

	session.AddHandler(bot.handleInteractionCreate)
	return bot, nil
}

func (b *discordBot) Open() error {
	if err := b.session.Open(); err != nil {
		return err
	}

	me, err := b.session.User("@me")
	if err != nil {
		b.session.Close()
		return err
	}

	_, err = b.session.ApplicationCommandCreate(
		me.ID,
		strconv.FormatUint(b.cfg.Discord.GuildID, 10),
		&discordgo.ApplicationCommand{
			Name:        verifyCommandName,
			Description: "Verify your umich.edu account",
		},
	)
	if err != nil {
		b.session.Close()
		return err
	}

	log.Printf("registered /%s for guild %d", verifyCommandName, b.cfg.Discord.GuildID)
	return nil
}

func (b *discordBot) Close() {
	if err := b.session.Close(); err != nil {
		log.Printf("discord close: %v", err)
	}
}

func (b *discordBot) handleInteractionCreate(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	if interaction.ApplicationCommandData().Name != verifyCommandName {
		return
	}

	if interaction.GuildID == "" || interaction.Member == nil || interaction.Member.User == nil {
		if err := respondEphemeral(session, interaction.Interaction, "This command must be used in a guild."); err != nil {
			log.Printf("interaction response: %v", err)
		}
		return
	}

	userID, err := strconv.ParseUint(interaction.Member.User.ID, 10, 64)
	if err != nil {
		log.Printf("parse user id: %v", err)
		if err := respondEphemeral(session, interaction.Interaction, "Could not generate a verification link."); err != nil {
			log.Printf("interaction response: %v", err)
		}
		return
	}

	guildID, err := strconv.ParseUint(interaction.GuildID, 10, 64)
	if err != nil {
		log.Printf("parse guild id: %v", err)
		if err := respondEphemeral(session, interaction.Interaction, "Could not generate a verification link."); err != nil {
			log.Printf("interaction response: %v", err)
		}
		return
	}

	authURL, err := b.authenticator.authURL(userID, guildID)
	if err != nil {
		log.Printf("generate auth url: %v", err)
		if err := respondEphemeral(session, interaction.Interaction, "Could not generate a verification link."); err != nil {
			log.Printf("interaction response: %v", err)
		}
		return
	}

	log.Printf("issued verification link for user_id=%d guild_id=%d", userID, guildID)
	if err := respondEphemeral(session, interaction.Interaction, fmt.Sprintf("Open this link to verify: %s", authURL)); err != nil {
		log.Printf("interaction response: %v", err)
	}
}

func respondEphemeral(session *discordgo.Session, interaction *discordgo.Interaction, content string) error {
	return session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (b *discordBot) addVerifiedRole(guildID uint64, userID uint64) error {
	err := b.session.GuildMemberRoleAdd(
		strconv.FormatUint(guildID, 10),
		strconv.FormatUint(userID, 10),
		strconv.FormatUint(b.cfg.Roles.Verified, 10),
	)
	if err != nil {
		return err
	}

	log.Printf("assigned verified role to user_id=%d guild_id=%d role_id=%d", userID, guildID, b.cfg.Roles.Verified)
	return nil
}
