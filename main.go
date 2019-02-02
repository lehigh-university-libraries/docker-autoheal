package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Config for autoheal options
type Config struct {
	Interval time.Duration
	Actions  []Action
}

func main() {
	cfg := Config{Actions: defaultActions()}

	flag.DurationVar(&cfg.Interval, "interval", 5*time.Second, "frequency interval")

	flag.Parse()
	log.SetLevel(log.DebugLevel)
	serve(&cfg)
}

func serve(config *Config) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	defer signal.Stop(sigint)

	cli, err := client.NewClientWithOpts(client.WithVersion("1.37"))
	if err != nil {
		panic(err)
	}

	unhealthyFilter := filters.NewArgs(filters.KeyValuePair{Key: "health", Value: "unhealthy"})

	healContainers := func(containers []types.Container, actions []Action) {
		for _, container := range containers {
			for _, action := range config.Actions {
				if err := action(&container); err != nil {
					log.Errorf(container.ID)
				}
			}
		}
	}

	for {
		log.Debug("check...")
		unhealthy, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
			Filters: unhealthyFilter,
		})
		if err != nil {
			panic(err)
		}

		healContainers(unhealthy, config.Actions)

		select {
		case <-time.After(config.Interval):
		case <-sigint:
			log.Println("cleanup...")
			os.Exit(0)
		}
	}
}
