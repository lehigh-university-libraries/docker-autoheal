package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Config for autoheal options
type Config struct {
	Interval    time.Duration
	LockFile    string
	WebhookUrl  string
	WebhookKey  string
	webhookLock bool
}

func main() {
	cfg := Config{
		webhookLock: false,
	}
	flag.DurationVar(&cfg.Interval, "interval", 10*time.Second, "frequency interval")
	flag.StringVar(&cfg.LockFile, "lock-file", "", "lock file that when exists halts docker autohealh")
	flag.StringVar(&cfg.WebhookUrl, "webhook-url", "", "Send messages to a webhook")
	flag.StringVar(&cfg.WebhookKey, "webhook-key", "text", "JSON key name to add webhook messages")

	flag.Parse()
	serve(&cfg)
}

func serve(config *Config) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	defer signal.Stop(sigint)

	cli, err := client.NewClientWithOpts(client.WithVersion("1.45"))
	if err != nil {
		panic(err)
	}

	unhealthyFilter := filters.NewArgs()
	unhealthyFilter.Add("health", "unhealthy")

	statusFilter := filters.NewArgs()
	statusFilter.Add("status", "exited")
	statusFilter.Add("status", "dead")

	stopOpts := container.StopOptions{}
	ctx := context.Background()
	slog.Info("Starting check")
	for {
		unhealthy, err := cli.ContainerList(ctx, container.ListOptions{
			Filters: unhealthyFilter,
		})
		if err != nil {
			slog.Error("Unable to list unhealthy docker containers", "err", err)
		}

		stopped, err := cli.ContainerList(ctx, container.ListOptions{
			Filters: statusFilter,
		})
		if err != nil {
			slog.Error("Unable to list stopped docker containers", "err", err)
		}

		containers := []string{}
		for _, c := range append(unhealthy, stopped...) {
			slog.Warn("Restarting container", "name", c.Names, "status", c.Status)
			if err := cli.ContainerRestart(ctx, c.ID, stopOpts); err != nil {
				slog.Error("Unable to restart container", "container.ID", c.ID, "err", err)
			}
			label := fmt.Sprintf("%s (%s)", strings.Join(c.Names, " "), c.Status)
			containers = append(containers, label)
		}

		if config.WebhookUrl != "" {
			if err := config.triggerWebhook(containers); err != nil {
				slog.Error("Unable to send to webhook", "err", err)
			}
		}

		select {
		case <-time.After(config.Interval):
		case <-sigint:
			slog.Info("Sigint detected. Exiting")
			os.Exit(0)
		}
	}
}

func (config *Config) triggerWebhook(labels []string) error {
	webhookMessage := ":white_check_mark: All is well"
	if len(labels) == 0 {
		if !config.webhookLock {
			return nil
		}
		config.webhookLock = false
	} else {
		config.webhookLock = true
		webhookMessage = fmt.Sprintf(`:rotating_light: Unhealthy services:
- %s`, strings.Join(labels, "\n-"))
	}
	payload := map[string]string{
		config.WebhookKey: webhookMessage,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", config.WebhookUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Received non-OK response: %s", resp.Status)
	}

	return nil
}
