package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/perbu/activity/internal/config"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

type Client struct {
	genaiClient *genai.Client
	model       string
}

// NewClient creates a new LLM client based on config
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	// Get API key from environment
	apiKey := os.Getenv(cfg.LLM.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in environment variable: %s", cfg.LLM.APIKeyEnv)
	}

	// Initialize GenAI client with Gemini API backend
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Client{
		genaiClient: client,
		model:       cfg.LLM.Model,
	}, nil
}

// Close is a no-op for genai.Client (no cleanup needed)
func (c *Client) Close() error {
	return nil
}

// GenerateText generates text from a prompt (non-streaming)
func (c *Client) GenerateText(ctx context.Context, prompt string) (string, error) {
	content := genai.NewContentFromText(prompt, genai.RoleUser)

	resp, err := c.genaiClient.Models.GenerateContent(ctx, c.model,
		[]*genai.Content{content},
		nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return resp.Text(), nil
}

// GetGeminiModel returns a model.LLM instance for use with ADK agents
func (c *Client) GetGeminiModel(ctx context.Context) (model.LLM, error) {
	// Get API key from environment (same as in NewClient)
	apiKey := os.Getenv("GOOGLE_API_KEY") // Using hardcoded env var for simplicity

	// Create a Gemini model using the ADK's gemini package
	llmModel, err := gemini.NewModel(ctx, c.model, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini model: %w", err)
	}
	return llmModel, nil
}
