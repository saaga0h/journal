package main

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	log := logger.New()

	godotenv.Load()

	cfg, err := config.Load("")
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	logger.SetLevel(cfg.Log.Level)

	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-brief-trigger-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT")
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	msg := mqttclient.BriefTrigger{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "nomad-periodic",
			Timestamp: time.Now().UTC(),
		},
	}

	if err := mqttClient.Publish(mqttclient.TopicBriefTrigger, msg); err != nil {
		log.WithError(err).Fatal("Failed to publish brief trigger")
	}

	log.Info("Published brief trigger")
}
