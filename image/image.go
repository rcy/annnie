package image

import (
	"context"
	"encoding/base64"
	"fmt"
	"goirc/db/model"
	"goirc/internal/ai"
	db "goirc/model"
	"log"
	"os"
	"strings"

	openaiofficial "github.com/openai/openai-go/v3"
)

func mustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is not set", key)
	}
	return value
}

var openaiAPIKey = mustGetenv("OPENAI_API_KEY")
var ImageFileBase = mustGetenv("IMAGE_FILE_BASE")
var rootURL = mustGetenv("ROOT_URL")

type GeneratedImage struct {
	model.GeneratedImage
}

func (gi *GeneratedImage) URL() string {
	return fmt.Sprintf("%s/i/%d", rootURL, gi.ID)
}

func GenerateGPTImage(ctx context.Context, prompt string) (*GeneratedImage, error) {
	client := openaiofficial.NewClient()

	imgResp, err := client.Images.Generate(ctx, openaiofficial.ImageGenerateParams{
		Prompt:  prompt,
		Model:   "gpt-image-2",
		N:       openaiofficial.Int(1),
		Quality: openaiofficial.ImageGenerateParamsQualityMedium,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "billing") {
			return nil, ai.ErrBilling
		}
		if strings.Contains(strings.ToLower(err.Error()), "rejected") {
			return nil, ai.ErrRejected
		}
		return nil, err
	}

	imgBytes, err := base64.StdEncoding.DecodeString(imgResp.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := model.New(tx)
	gi, err := q.CreateGeneratedImage(ctx, model.CreateGeneratedImageParams{
		Prompt:        prompt,
		RevisedPrompt: imgResp.Data[0].RevisedPrompt,
	})
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(ImageFileBase, os.FileMode(0755))
	if err != nil {
		return nil, err
	}

	imgFile, err := os.Create(fmt.Sprintf("%s/%d.png", ImageFileBase, gi.ID))
	if err != nil {
		return nil, err
	}
	defer imgFile.Close()

	_, err = imgFile.Write(imgBytes)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &GeneratedImage{gi}, nil
}
