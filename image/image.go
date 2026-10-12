package image

import (
	"context"
	"encoding/base64"
	"fmt"
	"goirc/config"
	"goirc/db/model"
	"goirc/internal/ai"
	db "goirc/model"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
)

type GeneratedImage struct {
	model.GeneratedImage
}

func (gi *GeneratedImage) URL() string {
	return fmt.Sprintf("%s/i/%d", config.Get().RootURL, gi.ID)
}

func GenerateGPTImage(ctx context.Context, prompt string) (*GeneratedImage, error) {
	client := openai.NewClient()

	imgResp, err := client.Images.Generate(ctx, openai.ImageGenerateParams{
		Prompt:     prompt,
		Model:      "gpt-image-2",
		N:          openai.Int(1),
		Quality:    openai.ImageGenerateParamsQualityMedium,
		Moderation: openai.ImageGenerateParamsModerationLow,
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

	err = os.MkdirAll(config.Get().ImageFileBase, os.FileMode(0755))
	if err != nil {
		return nil, err
	}

	imgFile, err := os.Create(fmt.Sprintf("%s/%d.png", config.Get().ImageFileBase, gi.ID))
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
