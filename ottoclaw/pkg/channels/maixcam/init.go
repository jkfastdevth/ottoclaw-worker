package maixcam

import (
	"github.com/sipeed/ottoclaw/pkg/bus"
	"github.com/sipeed/ottoclaw/pkg/channels"
	"github.com/sipeed/ottoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("maixcam", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewMaixCamChannel(cfg.Channels.MaixCam, b)
	})
}
