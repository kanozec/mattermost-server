// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/utils"
)

// EnsureBot implements Helpers.EnsureBot
func (p *HelpersImpl) EnsureBot(bot *model.Bot) (retBotID string, retErr error) {
	err := p.ensureServerVersion("5.10.0")
	if err != nil {
		return "", errors.Wrap(err, "failed to ensure bot")
	}

	// Must provide a bot with a username
	if bot == nil || len(bot.Username) < 1 {
		return "", errors.New("passed a bad bot, nil or no username")
	}

	// If we fail for any reason, this could be a race between creation of bot and
	// retrieval from another EnsureBot. Just try the basic retrieve existing again.
	defer func() {
		if retBotID == "" || retErr != nil {
			var err error
			var botIDBytes []byte

			err = utils.ProgressiveRetry(func() error {
				botIDBytes, err = p.API.KVGet(BOT_USER_KEY)
				if err != nil {
					return err
				}
				return nil
			})

			if err == nil && botIDBytes != nil {
				retBotID = string(botIDBytes)
				retErr = nil
			}
		}
	}()

	botIDBytes, kvGetErr := p.API.KVGet(BOT_USER_KEY)
	if kvGetErr != nil {
		return "", errors.Wrap(kvGetErr, "failed to get bot")
	}

	// If the bot has already been created, there is nothing to do.
	if botIDBytes != nil {
		botID := string(botIDBytes)
		return botID, nil
	}

	// Check for an existing bot user with that username. If one exists, then use that.
	if user, userGetErr := p.API.GetUserByUsername(bot.Username); userGetErr == nil && user != nil {
		if user.IsBot {
			if kvSetErr := p.API.KVSet(BOT_USER_KEY, []byte(user.Id)); kvSetErr != nil {
				p.API.LogWarn("Failed to set claimed bot user id.", "userid", user.Id, "err", kvSetErr)
			}
		} else {
			p.API.LogError("Plugin attempted to use an account that already exists. Convert user to a bot account in the CLI by running 'mattermost user convert <username> --bot'. If the user is an existing user account you want to preserve, change its username and restart the Mattermost server, after which the plugin will create a bot account with that name. For more information about bot accounts, see https://mattermost.com/pl/default-bot-accounts", "username", bot.Username, "user_id", user.Id)
		}
		return user.Id, nil
	}

	// Create a new bot user for the plugin
	createdBot, createBotErr := p.API.CreateBot(bot)
	if createBotErr != nil {
		return "", errors.Wrap(createBotErr, "failed to create bot")
	}

	if kvSetErr := p.API.KVSet(BOT_USER_KEY, []byte(createdBot.UserId)); kvSetErr != nil {
		p.API.LogWarn("Failed to set created bot user id.", "userid", createdBot.UserId, "err", kvSetErr)
	}

	return createdBot.UserId, nil
}
