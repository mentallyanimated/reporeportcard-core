package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/google/go-github/v41/github"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

type PR struct {
	info    *github.PullRequest
	reviews []*github.PullRequestReview
	files   []*github.CommitFile
}

func loadPRs() []*PR {
	prs := []*PR{}

	files, _ := ioutil.ReadDir("github-cache/color/color")
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		pr := &github.PullRequest{}
		infoContents, _ := ioutil.ReadFile("github-cache/color/color/" + file.Name())
		json.Unmarshal(infoContents, pr)

		reviews := []*github.PullRequestReview{}
		reviewsContent, _ := ioutil.ReadFile("github-cache/color/color/" + file.Name() + "/review.json")
		json.Unmarshal(reviewsContent, &reviews)

		files := []*github.CommitFile{}
		filesContent, _ := ioutil.ReadFile("github-cache/color/color/" + file.Name() + "/files.json")
		json.Unmarshal(filesContent, &files)

		prs = append(prs, &PR{
			info:    pr,
			reviews: reviews,
			files:   files,
		})
	}
	return prs
}

func main() {
	prs := loadPRs()

	userIDToLogin := map[int64]string{}
	edgeFrequency := map[simple.Edge]int{}
	totalApprovalCount := 0

	for _, pr := range prs {
		requestorID := pr.info.GetUser().GetID()
		requestorLogin := pr.info.GetUser().GetLogin()

		for _, review := range pr.reviews {
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
			W: float64(frequency) / float64(totalApprovalCount),
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

		forceGraphNodes = append(forceGraphNodes, forceGraphNode{
			ID:        userIDToLogin[id],
			Score:     adjustedRank,
			Neighbors: nodeToNeighbors[userIDToLogin[id]],
			Links:     links,
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
