package main

import (
	"context"
	"flag"
	"os"

	"github.com/mentallyanimated/reporeportcard-core/github"
	"github.com/mentallyanimated/reporeportcard-core/store"
)

// type PR struct {
// 	info    *github.PullRequest
// 	reviews []*github.PullRequestReview
// 	files   []*github.CommitFile
// }

// type ForceGraphLink struct {
// 	Source string `json:"source"`
// 	Target string `json:"target"`
// 	Value  int    `json:"value"`
// }

// type ForceGraphNode struct {
// 	ID        string           `json:"id"`
// 	Score     float64          `json:"score"`
// 	Neighbors []string         `json:"neighbors"`
// 	Links     []ForceGraphLink `json:"links"`
// 	Group     string           `json:"group"`
// }

// type ForceGraph struct {
// 	Nodes []ForceGraphNode `json:"nodes"`
// 	Links []ForceGraphLink `json:"links"`
// }

// func bin(min, max, value float64, numberOfBins int) int {
// 	binSize := (max - min) / float64(numberOfBins)
// 	bin := int(math.Floor((value - min) / binSize))
// 	if bin < 0 {
// 		bin = 0
// 	}
// 	if bin >= numberOfBins {
// 		bin = numberOfBins - 1
// 	}
// 	return bin
// }

// func loadPRs() []*PR {
// 	prs := []*PR{}

// 	files, _ := ioutil.ReadDir("github-cache/color/color")
// 	for _, file := range files {
// 		if file.IsDir() {
// 			continue
// 		}

// 		pr := &github.PullRequest{}
// 		infoContents, _ := ioutil.ReadFile("github-cache/color/color/" + file.Name())
// 		json.Unmarshal(infoContents, pr)

// 		reviews := []*github.PullRequestReview{}
// 		reviewsContent, _ := ioutil.ReadFile(fmt.Sprintf("github-cache/color/color/%d/review.json", pr.GetNumber()))
// 		json.Unmarshal(reviewsContent, &reviews)

// 		files := []*github.CommitFile{}
// 		filesContent, _ := ioutil.ReadFile(fmt.Sprintf("github-cache/color/color/%d/files.json", pr.GetNumber()))
// 		json.Unmarshal(filesContent, &files)

// 		prs = append(prs, &PR{
// 			info:    pr,
// 			reviews: reviews,
// 			files:   files,
// 		})
// 	}
// 	return prs
// }

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

	// prs := loadPRs()

	// userIDToLogin := map[int64]string{}
	// edgeFrequency := map[simple.Edge]int{}
	// totalApprovalCount := 0

	// for _, pr := range prs {
	// 	requestorID := pr.info.GetUser().GetID()
	// 	requestorLogin := pr.info.GetUser().GetLogin()

	// 	for _, review := range pr.reviews {
	// 		if review.GetState() != "APPROVED" {
	// 			continue
	// 		}
	// 		reviewerID := review.GetUser().GetID()
	// 		reviewerLogin := review.GetUser().GetLogin()

	// 		if requestorLogin == "" || reviewerLogin == "" {
	// 			continue
	// 		}

	// 		if requestorLogin == "ghost" || reviewerLogin == "ghost" {
	// 			continue
	// 		}

	// 		userIDToLogin[requestorID] = requestorLogin
	// 		userIDToLogin[reviewerID] = reviewerLogin

	// 		edge := simple.Edge{
	// 			F: simple.Node(requestorID),
	// 			T: simple.Node(reviewerID),
	// 		}

	// 		if _, ok := edgeFrequency[edge]; ok {
	// 			edgeFrequency[edge]++
	// 		} else {
	// 			edgeFrequency[edge] = 1
	// 		}

	// 		totalApprovalCount++
	// 	}
	// }

	// graph := simple.NewWeightedDirectedGraph(0, 0)

	// for edge, frequency := range edgeFrequency {
	// 	graph.SetWeightedEdge(simple.WeightedEdge{
	// 		F: edge.F,
	// 		T: edge.T,
	// 		W: (float64(frequency) / float64(totalApprovalCount)) * 100,
	// 	})
	// }

	// pageRank := network.PageRank(graph, 0.85, 0.00000001)
	// var minRankScore, maxRankScore float64

	// forceGraphNodes := []ForceGraphNode{}
	// forceGraphLinks := []ForceGraphLink{}
	// nodeToNeighbors := make(map[string][]string)

	// for edge := range edgeFrequency {
	// 	fLogin := userIDToLogin[edge.F.ID()]
	// 	tLogin := userIDToLogin[edge.T.ID()]

	// 	if _, ok := nodeToNeighbors[fLogin]; !ok {
	// 		nodeToNeighbors[fLogin] = []string{}
	// 	}
	// 	if _, ok := nodeToNeighbors[tLogin]; !ok {
	// 		nodeToNeighbors[tLogin] = []string{}
	// 	}

	// 	nodeToNeighbors[fLogin] = append(nodeToNeighbors[fLogin], tLogin)
	// }

	// for _, rank := range pageRank {
	// 	if rank < minRankScore || minRankScore == 0 {
	// 		minRankScore = rank
	// 	}
	// 	if rank > maxRankScore {
	// 		maxRankScore = rank
	// 	}
	// }

	// for id, rank := range pageRank {
	// 	links := []ForceGraphLink{}

	// 	for edge, freq := range edgeFrequency {
	// 		if edge.F.ID() == id {
	// 			links = append(links, ForceGraphLink{
	// 				Source: userIDToLogin[edge.F.ID()],
	// 				Target: userIDToLogin[edge.T.ID()],
	// 				Value:  freq,
	// 			})
	// 		}
	// 	}

	// 	adjustedRank := 10 * ((rank - minRankScore) / (maxRankScore - minRankScore))

	// 	group := 1
	// 	for rank < maxRankScore {
	// 		rank *= 2
	// 		group++
	// 	}

	// 	forceGraphNodes = append(forceGraphNodes, ForceGraphNode{
	// 		ID:        userIDToLogin[id],
	// 		Score:     adjustedRank,
	// 		Neighbors: nodeToNeighbors[userIDToLogin[id]],
	// 		Links:     links,
	// 		Group:     fmt.Sprintf("%d", group),
	// 	})
	// }

	// for edge, frequency := range edgeFrequency {
	// 	forceGraphLinks = append(forceGraphLinks, ForceGraphLink{
	// 		Source: userIDToLogin[edge.F.ID()],
	// 		Target: userIDToLogin[edge.T.ID()],
	// 		Value:  frequency,
	// 	})
	// }

	// forceGraph := ForceGraph{
	// 	Nodes: forceGraphNodes,
	// 	Links: forceGraphLinks,
	// }

	// f, _ := os.Create("force-graph.json")
	// defer f.Close()
	// json.NewEncoder(f).Encode(forceGraph)
}
