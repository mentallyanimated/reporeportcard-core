package main

import (
	"context"
	"flag"
	"os"

	"github.com/mentallyanimated/reporeportcard-core/github"
	"github.com/mentallyanimated/reporeportcard-core/server"
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

	server := server.NewServer()
	server.Start()

	// pullDetails := graph.ImportRawData(owner, repo)
	// filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, time.Unix(0, 0), time.Now())
	// // filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, time.Now().Add(-time.Hour*24*365), time.Now())

	// graph.BuildForceGraph(owner, repo, filteredPullDetails, os.Stdout)
}
