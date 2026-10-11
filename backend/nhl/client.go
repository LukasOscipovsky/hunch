package nhl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api-web.nhle.com/v1"

// Retry settings for rate-limited (429) responses.
const (
	maxRetries     = 3
	initialBackoff = 500 * time.Millisecond
)

// Client calls the public NHL web API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// StandingsNow returns the raw JSON of the current league standings.
func (c *Client) StandingsNow(ctx context.Context) ([]byte, error) {
	return c.get(ctx, "/standings/now")
}

// ScheduleNow returns the raw JSON of the current week's league schedule.
func (c *Client) ScheduleNow(ctx context.Context) ([]byte, error) {
	return c.get(ctx, "/schedule/now")
}

// CurrentRoster returns the raw JSON of a team's current roster.
func (c *Client) CurrentRoster(ctx context.Context, teamAbbrev string) ([]byte, error) {
	return c.get(ctx, "/roster/"+url.PathEscape(teamAbbrev)+"/current")
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	backoff := initialBackoff
	for attempt := 0; ; attempt++ {
		body, retryAfter, err := c.do(ctx, path)
		if err == nil || retryAfter < 0 || attempt == maxRetries {
			return body, err
		}

		wait := backoff
		if retryAfter > 0 {
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
}

// do performs a single request. On a 429 response it returns the server's
// Retry-After delay (0 if absent); otherwise retryAfter is -1.
func (c *Client) do(ctx context.Context, path string) (body []byte, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, -1, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, -1, fmt.Errorf("nhl request %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, fmt.Errorf("nhl read %s: %w", path, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter = 0
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			retryAfter = time.Duration(secs) * time.Second
		}
		return nil, retryAfter, fmt.Errorf("nhl %s: rate limited (status 429)", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, -1, fmt.Errorf("nhl %s: unexpected status %d", path, resp.StatusCode)
	}
	return body, -1, nil
}
