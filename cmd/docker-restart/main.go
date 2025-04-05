package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/ankel/handy-utils/pkg/log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const (
	RunLoop = 30 * time.Second
)

func main() {
	// Define and parse the log level flag.
	logLevelFlag := flag.String("log-level", "info", "Set the logging level (debug, info, warn, error)")
	flag.Parse()

	// Setup the l instance.
	l := log.New(*logLevelFlag)

	ctx := context.Background()

	l.Info("Starting docker-restar", "log-level", *logLevelFlag)

	// Create a new Docker client using environment variables.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(l, err, "Error creating Docker client")
	}
	defer cli.Close()

	for {
		// Sleep first has 2 advantages: 1) it ensure this container is always restarted, 2) `continue`ing at any point below will still get the right sleep
		time.Sleep(RunLoop)

		containers, err := cli.ContainerList(ctx, container.ListOptions{
			All: true,
		})
		if err != nil {
			l.Error("Error listing all containers", "error", err)
			continue
		}

		l.Debug("Found containers", "containers", buildContainerString(containers))
		// only restart 1 container per loop to avoid the thundering herd effect. But what if the same container keep crashing? Shuffle the list to avoid restarting the same one repeatedly
		rand.Shuffle(len(containers), func(i, j int) { containers[i], containers[j] = containers[j], containers[i] })

		for _, c := range containers {
			if c.State != "running" {
				if err := cli.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
					l.Error("Error restarting container", "error", err, "container", fmt.Sprintf("%#v", c))
				} else {
					l.Info("Restarted container", "container", fmt.Sprintf("%#v", c))
				}
				break
			}
		}
	}
}

func buildContainerString(containers []container.Summary) string {
	sb := strings.Builder{}

	for _, c := range containers {
		sb.WriteString(
			fmt.Sprintf("[%v:%v:%v], ", c.Names, c.State, c.Status))
	}

	return sb.String()
}
