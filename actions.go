package main

import (
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

// Action to perform during healing
type Action = func(c *types.Container) error

// ActionRestart restarts unhealthy container
func ActionRestart(c *types.Container) error {
	return nil
}

// ActionReboot reboots host
func ActionReboot(c *types.Container) error {
	return nil
}

// ActionOutput prints unhealthy container ID to stdout
func ActionOutput(c *types.Container) error {
	log.Printf("unhealthy container %s", c.ID)
	return nil
}

func defaultActions() []Action {
	return []Action{ActionOutput}
}
