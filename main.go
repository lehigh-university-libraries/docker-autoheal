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
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerAutoHeal struct {
	DockerCli       *client.Client
	Interval        time.Duration
	LockFile        string
	WebhookUrl      string
	WebhookKey      string
	ContainerStates map[string]*ContainerState
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	webhookLock     bool
}

type ContainerState struct {
	Backoff     time.Duration
	LastAttempt time.Time
}

func main() {
	cli, err := client.NewClientWithOpts(client.WithVersion("1.45"))
	if err != nil {
		slog.Error("Unable to initialize docker client", "err", err)
		os.Exit(1)
	}

	dah := DockerAutoHeal{
		DockerCli:       cli,
		webhookLock:     false,
		ContainerStates: make(map[string]*ContainerState),
	}
	flag.DurationVar(&dah.Interval, "interval", 10*time.Second, "how often to check for docker container health")
	flag.DurationVar(&dah.InitialBackoff, "initial-backoff", 10*time.Second, "how long to initially wait before restarting an unhealthy container")
	flag.DurationVar(&dah.MaxBackoff, "max-backoff", 300*time.Second, "maximum time to wait before attempting a container restart")
	flag.StringVar(&dah.LockFile, "lock-file", "", "lock file that when exists halts docker autohealh")
	flag.StringVar(&dah.WebhookUrl, "webhook-url", "", "Send messages to a webhook")
	flag.StringVar(&dah.WebhookKey, "webhook-key", "text", "JSON key name to add webhook messages")

	flag.Parse()
	dah.serve()
}

func (dah *DockerAutoHeal) serve() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting docker autoheal monitor", "interval", dah.Interval)
	t := time.NewTicker(dah.Interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			dah.checkHealth(ctx)
		case <-sc:
			slog.Info("Exiting")
			return
		}
	}
}

func (dah *DockerAutoHeal) checkHealth(ctx context.Context) {
	if dah.LockFile != "" {
		if _, err := os.Stat(dah.LockFile); err == nil {
			slog.Info("Lock file exists, not checking health")
			return
		}
	}

	cl, err := dah.DockerCli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		slog.Error("Unable to list docker containers", "err", err)
		return
	}

	unhealthyContainers := []string{}
	healthyContainers := []string{}
	for _, c := range cl {
		cn := strings.Join(c.Names, " ")
		if c.State == "running" {
			// marked healthy
			// OR doesn't have a healthcheck
			if !strings.Contains(c.Status, "unhealthy") {
				healthyContainers = append(healthyContainers, cn)
				continue
			}
		} else if c.State != "exited" && c.State != "dead" {
			continue
		}

		// this container is either exited, dead, or unhealthy

		state, exists := dah.ContainerStates[cn]
		if !exists {
			inspect, err := dah.DockerCli.ContainerInspect(ctx, c.ID)
			if err != nil {
				slog.Error("Unable to inspect container", "name", cn, "err", err)
				continue
			}
			finishedAt, err := time.Parse(time.RFC3339, inspect.State.FinishedAt)
			if err != nil {
				slog.Error("Unable to parse FinishedAt timestamp", "name", cn, "FinishedAt", inspect.State.FinishedAt, "err", err)
				continue
			}

			state = &ContainerState{
				Backoff:     dah.InitialBackoff,
				LastAttempt: finishedAt,
			}
			dah.ContainerStates[cn] = state
		}

		if time.Since(state.LastAttempt) < state.Backoff {
			slog.Warn("Skipping restart due to backoff", "name", cn, "backoff", state.Backoff)
			continue
		}

		slog.Warn("Restarting container", "name", cn)
		if err := dah.DockerCli.ContainerRestart(ctx, c.ID, container.StopOptions{}); err != nil {
			slog.Error("Unable to restart container", "container.Names", cn, "err", err)
		}

		// set the next backoff incase we run into this container again
		state.Backoff = min(state.Backoff*2, dah.MaxBackoff)
		state.LastAttempt = time.Now()

		label := fmt.Sprintf("%s (%s)", cn, c.Status)
		unhealthyContainers = append(unhealthyContainers, label)
	}

	dah.cleanupContainerStates(healthyContainers)
	if dah.WebhookUrl != "" {
		lockedPreWebhook := dah.webhookLock
		if err := dah.triggerWebhook(unhealthyContainers); err != nil {
			slog.Error("Unable to send to webhook", "err", err)
			// if the lock was set to true before triggering the webhook
			// and we failed to send the payload
			// make sure we always reset the lock on webhook failure to ensure
			// an all is well message will be sent when slack (or the webhook host)
			// comes back online
			if lockedPreWebhook {
				dah.webhookLock = true
			}
		}
	}
}

func (dah *DockerAutoHeal) triggerWebhook(labels []string) error {
	webhookMessage := ":white_check_mark: All is well"
	// if we had no failed containers
	if len(labels) == 0 {
		// if we are still in a failed state
		// do not send a webhook message
		if !dah.webhookLock || len(dah.ContainerStates) > 0 {
			return nil
		}

		dah.webhookLock = false
		// if we already sent a message
		// bail
	} else if dah.webhookLock {
		// TODO track containers messages were sent for
		// and if a now contaienr in the list, send it as well
		// for now, we know from slack one message means there is some container(s) failing
		// and they are broke until we get the all is well message
		return nil
	} else {
		dah.webhookLock = true
		webhookMessage = fmt.Sprintf(`:rotating_light: Unhealthy services:
    %s`, strings.Join(labels, "\n    "))
	}
	payload := map[string]string{
		dah.WebhookKey: webhookMessage,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", dah.WebhookUrl, bytes.NewBuffer(payloadBytes))
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
		return fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	return nil
}

// remove healthy containers from backoff monitor
func (dah *DockerAutoHeal) cleanupContainerStates(healthyContainers []string) {
	now := time.Now()
	for _, cn := range healthyContainers {
		if state, exists := dah.ContainerStates[cn]; exists {
			if now.Sub(state.LastAttempt) > dah.InitialBackoff {
				delete(dah.ContainerStates, cn)
			}
		}
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
