package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/go-github/v41/github"
	"github.com/mentallyanimated/reporeportcard-core/store"
	"golang.org/x/oauth2"
)

const (
	METADATA_KEY = "metadata"
)

// PullRequest is a type alias for github.PullRequest
type PullRequest = github.PullRequest

type PullRequestReview = github.PullRequestReview

type CommitFile = github.CommitFile

type PullDetails struct {
	PullRequest *PullRequest
	Reviews     []*PullRequestReview
	Files       []*CommitFile
}

// Metadata let's us store additional information about the data we're storing
// Such as when we might need or want to redownload data
type Metadata struct {
	LastModifiedTime time.Time `json:"lastModifiedTime"`
	LastPullNumber   int       `json:"lastPullNumber"`
}

type Client struct {
	cache  store.Store
	client *github.Client
	owner  string
	repo   string
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

func NewClient(ctx context.Context, token string, cache store.Store, owner, repo string) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauth2Client := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(oauth2Client)
	return &Client{
		cache:  cache,
		client: githubClient,
		owner:  owner,
		repo:   repo,
	}
}

func (c *Client) readOrCreateMetadata(ctx context.Context) (*Metadata, error) {
	metadata := &Metadata{
		// Never modified
		LastModifiedTime: time.Unix(0, 0).UTC(),
		LastPullNumber:   -1,
	}
	metadataContents, err := c.cache.Get(METADATA_KEY)
	if err != nil {
		if err == store.ErrNotFound {
			metadataBytes, err := json.Marshal(metadata)
			if err != nil {
				log.Printf("Error marshalling metadata: %v", err)
				return nil, errors.New("error marshalling metadata")
			}
			if err := c.cache.Put(METADATA_KEY, metadataBytes); err != nil {
				log.Printf("Error saving metadata: %v", err)
				return nil, errors.New("error saving metadata")
			}
		} else {
			log.Printf("Error getting metadata: %v", err)
		}
	} else {
		if err := json.Unmarshal(metadataContents, metadata); err != nil {
			log.Printf("Error unmarshalling metadata: %v", err)
			return nil, errors.New("error unmarshalling metadata")
		}
	}
	return metadata, nil
}

func (c *Client) updateMetadata(ctx context.Context, metadata *Metadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("Error marshalling metadata: %v", err)
		return errors.New("error marshalling metadata")
	}
	if err := c.cache.Put(METADATA_KEY, metadataBytes); err != nil {
		log.Printf("Error saving metadata: %v", err)
		return errors.New("error saving metadata")
	}
	return nil
}

func (c *Client) downloadReviews(ctx context.Context, pullNumber int) ([]*github.PullRequestReview, error) {
	allReviews := []*github.PullRequestReview{}
	opt := &github.ListOptions{}
	for {
		reviews, resp, err := c.client.PullRequests.ListReviews(ctx, c.owner, c.repo, pullNumber, opt)
		if err != nil {
			waitForRatelimit(resp)
			continue
		}
		allReviews = append(allReviews, reviews...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	reviewBytes, err := json.Marshal(allReviews)
	if err != nil {
		log.Printf("Error marshalling pull request review: %v", err)
		return nil, errors.New("error marshalling pull request review")
	}
	c.cache.Put(fmt.Sprintf("%d/reviews", pullNumber), reviewBytes)

	log.Printf("Downloaded pull requests reviews for %d", pullNumber)
	return allReviews, nil
}

func (c *Client) downloadFiles(ctx context.Context, pullNumber int) ([]*github.CommitFile, error) {
	allFiles := []*github.CommitFile{}
	opt := &github.ListOptions{}
	for {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, c.owner, c.repo, pullNumber, opt)
		if err != nil {
			waitForRatelimit(resp)
			continue
		}
		allFiles = append(allFiles, files...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	fileBytes, err := json.Marshal(allFiles)
	if err != nil {
		log.Printf("Error marshalling pull request files: %v", err)
		return nil, errors.New("error marshalling pull request files")
	}
	c.cache.Put(fmt.Sprintf("%d/files", pullNumber), fileBytes)

	log.Printf("Downloaded pull requests files for %d", pullNumber)
	return allFiles, nil
}

func (c *Client) DownloadPullDetails(ctx context.Context) error {
	metadata, err := c.readOrCreateMetadata(ctx)
	if err != nil {
		return err
	}

	// check metadata to see if we need to update

	WAIT_TIME := -time.Second * 20
	if metadata.LastModifiedTime.After(time.Now().Add(WAIT_TIME)) {
		log.Printf("LastModifiedTime is %v, not updating", metadata.LastModifiedTime)
		duration := -time.Now().Add(WAIT_TIME).Sub(metadata.LastModifiedTime)
		log.Printf("Will update in %v", duration)
		return nil
	}

	log.Printf("Metadata: %#v", metadata)

	allPullDetails := []*PullDetails{}
	opt := &github.PullRequestListOptions{}
	defer func(allPullDetails *[]*PullDetails) {
		if 0 < len(*allPullDetails) {
			c.updateMetadata(ctx, &Metadata{
				LastModifiedTime: time.Now().UTC(),
				LastPullNumber:   (*allPullDetails)[0].PullRequest.GetNumber(),
			})
		}
	}(&allPullDetails)

	for {
		pullRequests, resp, err := c.client.PullRequests.List(ctx, c.owner, c.repo, &github.PullRequestListOptions{
			State: "closed",
			ListOptions: github.ListOptions{
				Page:    opt.Page,
				PerPage: 100,
			},
		})
		if err != nil {
			waitForRatelimit(resp)
			continue
		}

		for _, pr := range pullRequests {
			if pr.GetNumber() <= metadata.LastPullNumber {
				log.Printf("Downloaded all pull requests up to previously last downloaded. Exiting early.")
				return nil
			}

			if pr.MergedAt != nil {
				prBytes, err := json.Marshal(pr)
				if err != nil {
					log.Printf("Error marshalling pull request: %v", err)
					return errors.New("error marshalling pull request")
				}
				c.cache.Put(fmt.Sprintf("%d", pr.GetNumber()), prBytes)

				reviews, err := c.downloadReviews(ctx, pr.GetNumber())
				if err != nil {
					log.Printf("Error downloading reviews: %v", err)
					return errors.New("error downloading reviews")
				}

				files, err := c.downloadFiles(ctx, pr.GetNumber())
				if err != nil {
					log.Printf("Error downloading files: %v", err)
					return errors.New("error downloading files")
				}

				allPullDetails = append(allPullDetails, &PullDetails{
					PullRequest: pr,
					Reviews:     reviews,
					Files:       files,
				})
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		log.Printf("Last downloaded: %d", allPullDetails[len(allPullDetails)-1].PullRequest.GetNumber())
	}
	log.Printf("Downloaded %d pull requests", len(allPullDetails))
	return nil
}
