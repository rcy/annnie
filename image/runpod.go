package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goirc/config"
	"goirc/db/model"
	db "goirc/model"
	"net/http"
	"os"
	"strings"
)

type runpodResponse struct {
	DelayTime     int    `json:"delayTime"`
	ExecutionType int    `json:"executionTime"`
	ID            string `json:"id"`
	Output        struct {
		ImageURL string   `json:"image_url"`
		Images   []string `json:"images"`
	} `json:"output"`
	Seed   int    `json:"seed"`
	Status string `json:"status"`
}

func GenerateRunpod(ctx context.Context, prompt string) (*GeneratedImage, error) {
	genResp, err := genRunpodImage(prompt)
	if err != nil {
		return nil, err
	}

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := model.New(tx)
	gi, err := q.CreateGeneratedImage(ctx, model.CreateGeneratedImageParams{
		Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(config.Get().ImageFileBase, os.FileMode(0755))
	if err != nil {
		return nil, err
	}

	err = decodeDataURL(genResp.Output.ImageURL, fmt.Sprintf("%s/%d.png", config.Get().ImageFileBase, gi.ID))
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &GeneratedImage{gi}, nil
}

func genRunpodImage(prompt string) (*runpodResponse, error) {
	client := &http.Client{}
	url := "https://api.runpod.ai/v2/rx6gph02422vep/runsync"

	payload, err := json.Marshal(map[string]any{
		"input": map[string]any{
			"prompt": prompt,
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+config.Get().RunPodAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	response := runpodResponse{}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func decodeDataURL(dataURL, outputFilePath string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL")
	}

	base64Data := parts[1]

	binaryData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("error decoding base64 data: %v", err)
	}

	err = os.WriteFile(outputFilePath, binaryData, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}
