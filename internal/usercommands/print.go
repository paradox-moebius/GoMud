package usercommands

import (
	"github.com/volte6/gomud/internal/events"
	"github.com/volte6/gomud/internal/rooms"
	"github.com/volte6/gomud/internal/users"
)

func Print(rest string, user *users.UserRecord, room *rooms.Room) (bool, error) {

	events.AddToQueue(events.Message{
		UserId: user.UserId,
		Text:   rest,
	})

	return true, nil
}
