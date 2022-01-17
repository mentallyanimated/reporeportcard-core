package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/mentallyanimated/reporeportcard-core/github"
	"github.com/mentallyanimated/reporeportcard-core/graph"
	"github.com/mentallyanimated/reporeportcard-core/store"
)

func main() {
	ownerFlag := flag.String("owner", "mentallyanimated", "The owner of the repository")
	repoFlag := flag.String("repo", "reporeportcard-core", "The repository to analyze")
	flag.Parse()

	if *ownerFlag == "" || *repoFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	owner := *ownerFlag
	repo := *repoFlag

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := store.NewDisk(owner, repo)
	client := github.NewClient(ctx, os.Getenv("GITHUB_TOKEN"), cache, owner, repo)
	client.DownloadPullDetails(ctx)

	pullDetails := graph.ImportRawData(owner, repo)
	// filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, time.Unix(0, 0), time.Now())
	filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, time.Now().Add(-time.Hour*24*14), time.Now())

	graph.BuildForceGraph(owner, repo, filteredPullDetails)
}
