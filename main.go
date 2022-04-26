package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/mentallyanimated/reporeportcard-core/github"
	"github.com/mentallyanimated/reporeportcard-core/graph"
	"github.com/mentallyanimated/reporeportcard-core/server"
	"github.com/mentallyanimated/reporeportcard-core/store"
)

func main() {
	ownerFlag := flag.String("owner", "mentallyanimated", "The owner of the repository")
	repoFlag := flag.String("repo", "reporeportcard-core", "The repository to analyze")
	serveFlag := flag.Bool("serve", false, "Set to true to serve the API")
	durationFlag := flag.Duration("duration", 60*24*time.Hour, "The duration of the analysis")
	flag.Parse()

	if *ownerFlag == "" || *repoFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *serveFlag {
		server := server.NewServer()
		server.Start()
	} else {
		owner := *ownerFlag
		repo := *repoFlag

		cache := store.NewDisk(owner, repo)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client := github.NewClient(ctx, os.Getenv("GITHUB_TOKEN"), cache, owner, repo)
		client.DownloadPullDetails(ctx)

		pullDetails := graph.ImportRawData(owner, repo)
		filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, time.Now().Add(-*durationFlag), time.Now())

		graph.BuildForceGraph(owner, repo, filteredPullDetails, os.Stdout)
	}
}
