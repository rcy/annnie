package bsky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"goirc/internal/responder"
	"io"
	"math/rand"
	"net/http"
	"net/url"
)

func Handle(params responder.Responder) error {
	url, err := getRandomLink(params.Match(1), params.Match(2))
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", url)

	return nil
}

type Post struct {
	Embed *struct {
		External *struct {
			URI *string `json:"uri"`
		} `json:"external"`
	} `json:"embed"`
}

type Response struct {
	Posts []Post `json:"posts"`
}

func getRandomLink(domain string, query string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.feed.searchPosts?domain=%s&sort=latest&limit=3&q=%s",
		domain,
		url.QueryEscape(query)), nil)
	if err != nil {
		return "", err
	}

	//req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "github.com/rcy/annnie")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	var uris []string
	for _, post := range response.Posts {
		if post.Embed != nil && post.Embed.External != nil && post.Embed.External.URI != nil {
			uris = append(uris, *post.Embed.External.URI)
		}
	}

	if len(uris) == 0 {
		return "", fmt.Errorf("No URIs found")
	}

	return uris[rand.Intn(len(uris))], nil
}

// ///////////////////////////////////////////////////////////////// session
type SessionRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type SessionResponse struct {
	AccessJWT  string `json:"accessJwt"`
	RefreshJWT string `json:"refreshJwt"`
	DID        string `json:"did"`
	Handle     string `json:"handle"`
}

func getSession(identifier, password string) (*SessionResponse, error) {
	reqBody := SessionRequest{
		Identifier: identifier,
		Password:   password,
	}

	// Encode request to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Create POST request to the PDS
	url := "https://bsky.social/xrpc/com.atproto.server.createSession"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "my-go-client/0.1")

	// Do the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode JSON response
	var sessionResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return nil, err
	}

	return &sessionResp, nil
}
