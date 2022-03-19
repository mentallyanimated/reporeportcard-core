package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mentallyanimated/reporeportcard-core/github"
	"github.com/mentallyanimated/reporeportcard-core/store"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

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
	Group     string           `json:"group"`
}

type forceGraph struct {
	Nodes []forceGraphNode `json:"nodes"`
	Links []forceGraphLink `json:"links"`
}

// ImportRawData assumes that you've downloaded the data from the github API
// already and that it exists on disk. This will load every single pull request
// into memory.
func ImportRawData(owner, repo string) []*github.PullDetails {
	rootDirName := fmt.Sprintf("%s/%s/%s", store.CACHE_PREFIX, owner, repo)

	fileInfos, err := ioutil.ReadDir(rootDirName)
	if err != nil {
		log.Printf("Error reading directory: %s", err)
		return nil
	}

	filteredFileInfos := []os.FileInfo{}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		if strings.EqualFold(fmt.Sprintf("%s.json", github.METADATA_KEY), fileInfo.Name()) {
			continue
		}

		filteredFileInfos = append(filteredFileInfos, fileInfo)
	}

	allPullDetails := make([]*github.PullDetails, len(filteredFileInfos), len(filteredFileInfos))

	type FileInfoTuple struct {
		Index    int
		FileInfo os.FileInfo
	}

	fileInfoCh := make(chan *FileInfoTuple)
	go func() {
		for i, fileInfo := range filteredFileInfos {
			fileInfoCh <- &FileInfoTuple{i, fileInfo}
		}
		close(fileInfoCh)
	}()

	var wg sync.WaitGroup
	for fileInfoTuple := range fileInfoCh {
		i, fileInfo := fileInfoTuple.Index, fileInfoTuple.FileInfo
		wg.Add(1)
		go func(i int, fileInfo os.FileInfo) {
			pullBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", rootDirName, fileInfo.Name()))
			if err != nil {
				log.Printf("Error parsing pull details: %s", err)
				return
			}

			var pull *github.PullRequest
			err = json.Unmarshal(pullBytes, &pull)
			if err != nil {
				log.Printf("Error parsing pull: %s", err)
				return
			}

			reviewBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%d/reviews.json", rootDirName, pull.GetNumber()))
			if err != nil {
				log.Printf("Error reading reviews: %s", err)
				return
			}

			var reviews []*github.PullRequestReview
			err = json.Unmarshal(reviewBytes, &reviews)
			if err != nil {
				log.Printf("Error parsing reviews: %s", err)
				return
			}

			filesBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%d/files.json", rootDirName, pull.GetNumber()))
			if err != nil {
				log.Printf("Error reading files: %s", err)
				return
			}

			var files []*github.CommitFile
			err = json.Unmarshal(filesBytes, &files)
			if err != nil {
				log.Printf("Error parsing files: %s", err)
				return
			}

			pullDetails := &github.PullDetails{
				PullRequest: pull,
				Reviews:     reviews,
				Files:       files,
			}
			allPullDetails[i] = pullDetails
			wg.Done()
		}(i, fileInfo)
	}

	wg.Wait()

	return allPullDetails
}

// FilterPullDetailsByTime does a client side filtering of the pull details
func FilterPullDetailsByTime(pullDetails []*github.PullDetails, start, end time.Time) []*github.PullDetails {
	// TODO: Handle unspecified start/end times

	filteredPullDetails := []*github.PullDetails{}

	// Sort by GetCreatedAt
	sort.Slice(pullDetails, func(i, j int) bool {
		return pullDetails[i].PullRequest.GetCreatedAt().Before(pullDetails[j].PullRequest.GetCreatedAt())
	})

	for _, pullDetail := range pullDetails {
		if pullDetail.PullRequest.GetCreatedAt().Before(start) {
			continue
		}
		if pullDetail.PullRequest.GetCreatedAt().After(end) {
			break
		}
		filteredPullDetails = append(filteredPullDetails, pullDetail)
	}

	return filteredPullDetails
}

func BuildForceGraph(owner, repo string, pullDetails []*github.PullDetails, w io.Writer) {
	log.Printf("Building force graph for %s/%s out of %d pull requests", owner, repo, len(pullDetails))

	userIDToLogin := map[int64]string{}
	edgeFrequency := map[simple.Edge]int{}
	totalApprovalCount := 0

	for _, pullDetail := range pullDetails {
		requestorID := pullDetail.PullRequest.GetUser().GetID()
		requestorLogin := pullDetail.PullRequest.GetUser().GetLogin()

		for _, review := range pullDetail.Reviews {
			if review.GetState() != "APPROVED" {
				continue
			}
			reviewerID := review.GetUser().GetID()
			reviewerLogin := review.GetUser().GetLogin()

			if requestorLogin == "" || reviewerLogin == "" {
				continue
			}

			if requestorLogin == "ghost" || reviewerLogin == "ghost" {
				continue
			}

			userIDToLogin[requestorID] = requestorLogin
			userIDToLogin[reviewerID] = reviewerLogin

			edge := simple.Edge{
				F: simple.Node(requestorID),
				T: simple.Node(reviewerID),
			}

			if _, ok := edgeFrequency[edge]; ok {
				edgeFrequency[edge]++
			} else {
				edgeFrequency[edge] = 1
			}

			totalApprovalCount++
		}
	}

	graph := simple.NewWeightedDirectedGraph(0, 0)

	for edge, frequency := range edgeFrequency {
		graph.SetWeightedEdge(simple.WeightedEdge{
			F: edge.F,
			T: edge.T,
			W: (float64(frequency) / float64(totalApprovalCount)) * 100,
		})
	}

	pageRank := network.PageRank(graph, 0.85, 0.00000001)
	var minRankScore, maxRankScore float64

	forceGraphNodes := []forceGraphNode{}
	forceGraphLinks := []forceGraphLink{}
	nodeToNeighbors := make(map[string][]string)

	for edge := range edgeFrequency {
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

	for _, rank := range pageRank {
		if rank < minRankScore || minRankScore == 0 {
			minRankScore = rank
		}
		if rank > maxRankScore {
			maxRankScore = rank
		}
	}

	for id, rank := range pageRank {
		links := []forceGraphLink{}

		for edge, freq := range edgeFrequency {
			if edge.F.ID() == id {
				links = append(links, forceGraphLink{
					Source: userIDToLogin[edge.F.ID()],
					Target: userIDToLogin[edge.T.ID()],
					Value:  freq,
				})
			}
		}

		adjustedRank := 10 * ((rank - minRankScore) / (maxRankScore - minRankScore))

		group := 1
		for rank < maxRankScore {
			rank *= 2
			group++
		}

		forceGraphNodes = append(forceGraphNodes, forceGraphNode{
			ID:        userIDToLogin[id],
			Score:     adjustedRank,
			Neighbors: nodeToNeighbors[userIDToLogin[id]],
			Links:     links,
			Group:     fmt.Sprintf("%d", group),
		})
	}

	for edge, frequency := range edgeFrequency {
		forceGraphLinks = append(forceGraphLinks, forceGraphLink{
			Source: userIDToLogin[edge.F.ID()],
			Target: userIDToLogin[edge.T.ID()],
			Value:  frequency,
		})
	}

	forceGraph := forceGraph{
		Nodes: forceGraphNodes,
		Links: forceGraphLinks,
	}

	json.NewEncoder(w).Encode(forceGraph)
}
