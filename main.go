package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v41/github"
	"github.com/peterbourgon/diskv/v3"
	"golang.org/x/oauth2"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

type reviewTuple struct {
	PR     *github.PullRequest
	Review *github.PullRequestReview
}

type fileTuple struct {
	PR   *github.PullRequest
	File *github.CommitFile
}

type forceGraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int    `json:"value"`
}

type forceGraphNode struct {
	ID        string           `json:"id"`
	Score     float64          `json:"score"`
	Neighbors []string         `json:"neighbors"`
	Links     []forceGraphLink `json:"links"`
}

type forceGraph struct {
	Nodes []forceGraphNode `json:"nodes"`
	Links []forceGraphLink `json:"links"`
}

func FolderTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".json",
	}
}

func InverseFolderTransform(pathKey *diskv.PathKey) (key string) {
	j := pathKey.FileName[len(pathKey.FileName)-4:]
	if j != ".json" {
		panic("Invalid file found in storage folder!")
	}
	return strings.Join(pathKey.Path, "/") + pathKey.FileName[:len(pathKey.FileName)-5]
}

func waitForRatelimit(r *github.Response) {
	if r.StatusCode == 403 {
		for time.Now().Before(r.Rate.Reset.Time) {
			log.Printf("API Rate limit exceeded. Sleeping for %v", r.Rate.Reset.Sub(time.Now()))
			time.Sleep(time.Minute * 5)
		}

		retryAfter := r.Header.Get("Retry-After")
		if retryAfter != "" {
			wait, _ := strconv.Atoi(retryAfter)
			log.Printf("Abuse rate limit exceeded. Sleeping for %v", time.Duration(wait)*time.Second)
			time.Sleep(time.Duration(wait) * time.Second)
		}
	}
}

func main() {
	ctx := context.Background()

	var owner, repo string
	var startPR, endPR int
	flag.StringVar(&owner, "owner", "", "Github owner")
	flag.StringVar(&repo, "repo", "", "Github repo")
	flag.IntVar(&startPR, "startPR", -1, "Start PR")
	flag.IntVar(&endPR, "endPR", -1, "End PR")
	flag.Parse()

	// check if owner and repo are set
	if owner == "" || repo == "" {
		log.Fatal("Please provide owner and repo")
	}

	// check if startPR and endPR are set
	if startPR == -1 || endPR == -1 {
		log.Fatal("Please provide startPR and endPR")
	}

	d := diskv.New(diskv.Options{
		BasePath:          "github-cache",
		CacheSizeMax:      1024 * 1024 * 512,
		AdvancedTransform: FolderTransform,
		InverseTransform:  InverseFolderTransform,
	})

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	pullIDsChan := make(chan int)
	pulls := make(chan *github.PullRequest)
	reviewTupleChan := make(chan reviewTuple)
	fileTupleChan := make(chan fileTuple)

	const concurrency = 4
	var pullsProducerWG sync.WaitGroup
	var reviewsWG sync.WaitGroup

	go func() {
		for i := startPR; i < endPR; i++ {
			pullIDsChan <- i
		}
		close(pullIDsChan)
	}()

	pullsProducerWG.Add(concurrency)
	go func() {
		for i := 0; i < concurrency; i++ {
			go func(i int) {
				defer pullsProducerWG.Done()
				for pullID := range pullIDsChan {
					key := fmt.Sprintf("%s/%s/%d", owner, repo, pullID)

					var pull *github.PullRequest
					if d.Has(key) {
						b, _ := d.Read(key)
						json.Unmarshal(b, &pull)
						pulls <- pull
					} else {
						log.Printf("Fetching %s", key)
						pullRequest, resp, err := client.PullRequests.Get(ctx, owner, repo, pullID)
						if err != nil {
							waitForRatelimit(resp)
							continue
						}

						b, _ := json.Marshal(pullRequest)
						d.Write(key, b)
						pulls <- pullRequest
					}
				}
			}(i)
		}
		pullsProducerWG.Wait()
		close(pulls)
	}()

	reviewsWG.Add(concurrency)
	go func() {
		for i := 0; i < concurrency; i++ {
			go func(i int) {
				defer reviewsWG.Done()
				for pull := range pulls {
					reviewKey := fmt.Sprintf("%s/%s/%d/review", owner, repo, pull.GetNumber())
					allReviews := make([]*github.PullRequestReview, 0)

					if !d.Has(reviewKey) {
						log.Printf("Fetching %s", reviewKey)
						listReviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, pull.GetNumber(), nil)
						if err != nil {
							waitForRatelimit(resp)
							continue
						}
						b, _ := json.Marshal(listReviews)
						d.Write(reviewKey, b)
						allReviews = append(allReviews, listReviews...)
					} else {
						b, _ := d.Read(reviewKey)
						var listReviews []*github.PullRequestReview
						json.Unmarshal(b, &listReviews)
						allReviews = append(allReviews, listReviews...)
					}

					for _, review := range allReviews {
						reviewTupleChan <- reviewTuple{pull, review}
					}

					filesKey := fmt.Sprintf("%s/%s/%d/files", owner, repo, pull.GetNumber())
					if !d.Has(filesKey) {
						log.Printf("Fetching %s", filesKey)
						allFiles := []*github.CommitFile{}
						opt := &github.ListOptions{}
						for {
							listFiles, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, pull.GetNumber(), opt)
							if err != nil {
								waitForRatelimit(resp)
								continue
							}
							allFiles = append(allFiles, listFiles...)
							if resp.NextPage == 0 {
								break
							}
							opt.Page = resp.NextPage
						}
						b, _ := json.Marshal(allFiles)
						d.Write(filesKey, b)

						// for _, file := range allFiles {
						// 	fileTupleChan <- fileTuple{pull, file}
						// }
					} else {
						b, _ := d.Read(filesKey)
						var allFiles []*github.CommitFile
						json.Unmarshal(b, &allFiles)

						// for _, file := range allFiles {
						// 	fileTupleChan <- fileTuple{pull, file}
						// }
					}
				}
			}(i)
		}
		reviewsWG.Wait()
		close(reviewTupleChan)
		close(fileTupleChan)
	}()

	userIDToLogin := make(map[int64]string)

	totalApprovals := 0
	edgeFreq := make(map[simple.Edge]int)

	for review := range reviewTupleChan {
		if review.Review.GetState() != "APPROVED" {
			continue
		}

		requestorID := review.PR.GetUser().GetID()
		requestorLogin := review.PR.GetUser().GetLogin()
		reviewerID := review.Review.GetUser().GetID()
		reviewerLogin := review.Review.GetUser().GetLogin()

		if requestorLogin == "" || reviewerLogin == "" {
			continue
		}

		if requestorLogin != "" {
			userIDToLogin[requestorID] = requestorLogin
		}
		if reviewerLogin != "" {
			userIDToLogin[reviewerID] = reviewerLogin
		}

		edge := simple.Edge{
			F: simple.Node(requestorID),
			T: simple.Node(reviewerID),
		}

		if _, ok := edgeFreq[edge]; ok {
			edgeFreq[edge]++
		} else {
			edgeFreq[edge] = 1
		}

		totalApprovals++
	}

	weightedGraph := simple.NewWeightedDirectedGraph(0, 0)
	unweightedGraph := simple.NewDirectedGraph()

	for k, v := range edgeFreq {
		weightedGraph.SetWeightedEdge(simple.WeightedEdge{
			F: k.F,
			T: k.T,
			W: float64(v) / float64(totalApprovals),
		})
		unweightedGraph.SetEdge(simple.Edge{
			F: k.F,
			T: k.T,
		})
	}
	log.Printf("%d total approvals", totalApprovals)

	weightedPageRank := network.PageRank(weightedGraph, 0.85, 0.00000001)
	unweightedPageRank := network.PageRank(unweightedGraph, 0.85, 0.00000001)

	type tuple struct {
		ID   int64
		Rank float64
	}

	weightedTuples := make([]tuple, len(weightedPageRank))
	unweightedTuples := make([]tuple, len(unweightedPageRank))

	i := 0
	for k, v := range weightedPageRank {
		weightedTuples[i] = tuple{
			ID:   k,
			Rank: v,
		}
		i++
	}

	i = 0
	for k, v := range unweightedPageRank {
		unweightedTuples[i] = tuple{
			ID:   k,
			Rank: v,
		}
		i++
	}

	// sort tuples
	sort.Slice(weightedTuples, func(i, j int) bool {
		return weightedTuples[i].Rank < weightedTuples[j].Rank
	})
	sort.Slice(unweightedTuples, func(i, j int) bool {
		return unweightedTuples[i].Rank < unweightedTuples[j].Rank
	})

	log.Println()
	log.Println("Weighted PageRank (normalized approvals frequency)")
	log.Println("~~~~~~~~~~~~~~~~~")

	for i := 0; i < 5; i++ {
		tup := weightedTuples[len(weightedTuples)-1-i]
		log.Printf("%s: %.5f", userIDToLogin[tup.ID], tup.Rank)
	}

	log.Println()
	log.Println("Unweighted PageRank")
	log.Println("~~~~~~~~~~~~~~~~~~~")

	for i := 0; i < 5; i++ {
		tup := unweightedTuples[len(unweightedTuples)-1-i]
		log.Printf("%s: %.5f", userIDToLogin[tup.ID], tup.Rank)
	}

	forceGraphNodes := []forceGraphNode{}
	forceGraphLinks := []forceGraphLink{}
	nodeToNeighbors := make(map[string][]string)

	for edge := range edgeFreq {
		fLogin := userIDToLogin[edge.F.ID()]
		tLogin := userIDToLogin[edge.T.ID()]

		if _, ok := nodeToNeighbors[fLogin]; !ok {
			nodeToNeighbors[fLogin] = []string{}
		}
		if _, ok := nodeToNeighbors[tLogin]; !ok {
			nodeToNeighbors[tLogin] = []string{}
		}

		nodeToNeighbors[fLogin] = append(nodeToNeighbors[fLogin], tLogin)
	}

	minScore := weightedTuples[0].Rank
	maxScore := weightedTuples[len(unweightedTuples)-1].Rank

	for _, tup := range weightedTuples {
		links := []forceGraphLink{}

		for edge, freq := range edgeFreq {
			if edge.F.ID() == tup.ID {
				links = append(links, forceGraphLink{
					Source: userIDToLogin[edge.F.ID()],
					Target: userIDToLogin[edge.T.ID()],
					Value:  freq,
				})
			}
		}

		score := (tup.Rank - minScore) / (maxScore - minScore)
		log.Printf("User %s; Rank %f; Min %f; Max %f; Adjusted score %f", userIDToLogin[tup.ID], tup.Rank, minScore, maxScore, score)

		forceGraphNodes = append(forceGraphNodes, forceGraphNode{
			ID:        userIDToLogin[tup.ID],
			Score:     score * 10,
			Neighbors: nodeToNeighbors[userIDToLogin[tup.ID]],
			Links:     links,
		})
	}

	for edge, freq := range edgeFreq {
		forceGraphLinks = append(forceGraphLinks, forceGraphLink{
			Source: userIDToLogin[edge.F.ID()],
			Target: userIDToLogin[edge.T.ID()],
			Value:  freq,
		})
	}

	forceGraph := forceGraph{
		Nodes: forceGraphNodes,
		Links: forceGraphLinks,
	}

	// io.Writer for file
	f, _ := os.Create("force-graph.json")
	defer f.Close()
	json.NewEncoder(f).Encode(forceGraph)
}
