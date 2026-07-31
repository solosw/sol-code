package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const DefaultModel = "claude-opus-4-8"
const DefaultMaxRetries = 5

// Wire formats for outbound chat requests.
const (
	// FormatAnthropic sends native Anthropic Messages API requests (default).
	FormatAnthropic = "anthropic"
	// FormatOpenAI sends OpenAI chat/completions-compatible requests to
	// {base_url}/chat/completions. Internal engine types stay Anthropic SDK shaped.
	FormatOpenAI = "openai"
)

// Options configures the API client wrapper.
type Options struct {
	APIKey  string
	BaseURL string
	// Format selects the outbound protocol. Empty defaults to anthropic.
	Format string
}

// Client is a thin wrapper around the official Anthropic Go SDK, with an optional
// OpenAI chat/completions transport for compatible gateways.
type Client struct {
	sdk     sdk.Client
	format  string
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewClient(opts Options) *Client {
	format := NormalizeFormat(opts.Format)
	requestOptions := make([]option.RequestOption, 0, 3)
	requestOptions = append(requestOptions, option.WithMaxRetries(DefaultMaxRetries))
	if opts.APIKey != "" {
		requestOptions = append(requestOptions, option.WithAPIKey(opts.APIKey))
	}
	if opts.BaseURL != "" {
		requestOptions = append(requestOptions, option.WithBaseURL(opts.BaseURL))
	}
	return &Client{
		sdk:     sdk.NewClient(requestOptions...),
		format:  format,
		apiKey:  opts.APIKey,
		baseURL: strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		http:    &http.Client{Timeout: 0}, // stream can be long-lived
	}
}

// NormalizeFormat returns a supported wire format; unknown values fall back to anthropic.
func NormalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatOpenAI, "chat", "chat_completions", "chat-completions":
		return FormatOpenAI
	default:
		return FormatAnthropic
	}
}

func (c *Client) Format() string {
	if c == nil {
		return FormatAnthropic
	}
	return NormalizeFormat(c.format)
}

func (c *Client) SDK() *sdk.Client {
	if c == nil {
		return nil
	}
	return &c.sdk
}

// Create sends one chat request. When Format is openai, it uses OpenAI
// chat/completions at {base_url}/chat/completions and maps the response back
// into an Anthropic sdk.Message. When Stream is true, deltas are delivered via
// OnTextDelta / OnThinkingDelta callbacks.
func (c *Client) Create(ctx context.Context, req MessageRequest) (*sdk.Message, error) {
	if c == nil {
		return nil, fmt.Errorf("anthropic client is nil")
	}
	if c.Format() == FormatOpenAI {
		return c.createOpenAI(ctx, req)
	}
	params := req.ToSDKParams()
	if !req.Stream {
		return c.sdk.Messages.New(ctx, params)
	}

	stream := c.sdk.Messages.NewStreaming(ctx, params)
	message := sdk.Message{}
	for stream.Next() {
		event := stream.Current()
		dispatchStreamCallbacks(req, event)
		if err := message.Accumulate(event); err != nil {
			return nil, err
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &message, nil
}

// idleHTTPTimeout is only used for non-stream OpenAI requests.
func (c *Client) idleHTTPClient() *http.Client {
	if c == nil || c.http == nil {
		return &http.Client{Timeout: 10 * time.Minute}
	}
	return &http.Client{Timeout: 10 * time.Minute, Transport: c.http.Transport}
}
