package graph

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

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

// importRawData assumes that you've downloaded the data from the github API already and that it exists on disk.
func importRawData(owner, repo string) []*github.PullDetails {
	rootDirName := fmt.Sprintf("%s/%s/%s", store.CACHE_PREFIX, owner, repo)
	allPullDetails := []*github.PullDetails{}

	fileInfos, err := ioutil.ReadDir(rootDirName)
	if err != nil {
		log.Printf("Error reading directory: %s", err)
		return nil
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		if strings.EqualFold(fmt.Sprintf("%s.json", github.METADATA_KEY), fileInfo.Name()) {
			continue
		}

		log.Printf("Reading file: %s", fileInfo.Name())
		pullBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", rootDirName, fileInfo.Name()))
		if err != nil {
			log.Printf("Error parsing pull details: %s", err)
			return nil
		}

		var pull *github.PullRequest
		err = json.Unmarshal(pullBytes, &pull)
		if err != nil {
			log.Printf("Error parsing pull: %s", err)
			return nil
		}

		reviewBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%d/reviews.json", rootDirName, pull.GetNumber()))
		if err != nil {
			log.Printf("Error reading reviews: %s", err)
			return nil
		}
		log.Printf("%v", string(reviewBytes))

		var reviews []*github.PullRequestReview
		err = json.Unmarshal(reviewBytes, &reviews)
		if err != nil {
			log.Printf("Error parsing reviews: %s", err)
			return nil
		}

		filesBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/%d/files.json", rootDirName, pull.GetNumber()))
		if err != nil {
			log.Printf("Error reading files: %s", err)
			return nil
		}

		var files []*github.CommitFile
		err = json.Unmarshal(filesBytes, &files)
		if err != nil {
			log.Printf("Error parsing files: %s", err)
			return nil
		}

		pullDetails := &github.PullDetails{
			PullRequest: pull,
			Reviews:     reviews,
			Files:       files,
		}
		allPullDetails = append(allPullDetails, pullDetails)
	}

	return allPullDetails
}

func BuildForceGraph(owner, repo string) {
	prs := importRawData(owner, repo)
	userIDToLogin := map[int64]string{}
	edgeFrequency := map[simple.Edge]int{}
	totalApprovalCount := 0

	for _, pr := range prs {
		requestorID := pr.PullRequest.GetUser().GetID()
		requestorLogin := pr.PullRequest.GetUser().GetLogin()

		for _, review := range pr.Reviews {
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

	f, _ := os.Create("force-graph.json")
	defer f.Close()
	json.NewEncoder(f).Encode(forceGraph)
}
