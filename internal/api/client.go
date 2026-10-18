package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mpvt/internal/config"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	GlobalCfg  config.GlobalConfig
}

func NewClient(cfg *config.Config) *Client {
	endpoint := cfg.GetEffectiveEndpoint()
	endpoint = strings.TrimRight(endpoint, "/")

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if len(via) > 0 {
				isCrossHost := req.URL.Host != via[0].URL.Host
				isSchemeDowngrade := via[0].URL.Scheme == "https" && req.URL.Scheme == "http"
				if isCrossHost || isSchemeDowngrade {
					req.Header.Del("x-internal-token")
					req.Header.Del("x-user-id")
					req.Header.Del("x-user-role")
					req.Header.Del("x-user-email")
					req.Header.Del("Authorization")
				}
			}
			return nil
		},
	}

	return &Client{
		BaseURL:    endpoint,
		HTTPClient: httpClient,
		GlobalCfg:  cfg.Global,
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	fullPath := path
	if strings.HasSuffix(c.BaseURL, "/api/v1") && strings.HasPrefix(path, "/api/v1") {
		fullPath = strings.TrimPrefix(path, "/api/v1")
	}
	fullURL := c.BaseURL + fullPath
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Apply authentication headers
	// 1. Gateway header mode if internal token is provided
	if c.GlobalCfg.InternalToken != "" && c.GlobalCfg.UserId != "" {
		req.Header.Set("x-internal-token", c.GlobalCfg.InternalToken)
		req.Header.Set("x-user-id", c.GlobalCfg.UserId)
		if c.GlobalCfg.UserRole != "" {
			req.Header.Set("x-user-role", c.GlobalCfg.UserRole)
		} else {
			req.Header.Set("x-user-role", "USER")
		}
		if c.GlobalCfg.UserEmail != "" {
			req.Header.Set("x-user-email", c.GlobalCfg.UserEmail)
		}
	} else if c.GlobalCfg.Token != "" {
		// 2. Direct Bearer token mode
		token := c.GlobalCfg.Token
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		req.Header.Set("Authorization", token)
	}

	return req, nil
}

func (c *Client) do(req *http.Request, target any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed (%s %s): %w", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		limitedReader := io.LimitReader(resp.Body, 64*1024)
		respBytes, err := io.ReadAll(limitedReader)
		if err != nil {
			return fmt.Errorf("failed to read error response: %w", err)
		}

		var errResp ErrorResponse
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error.Code != "" {
			apiErr := errResp.Error
			apiErr.StatusCode = resp.StatusCode
			return &apiErr
		}
		var directErr APIError
		if jsonErr := json.Unmarshal(respBytes, &directErr); jsonErr == nil && directErr.Code != "" {
			directErr.StatusCode = resp.StatusCode
			return &directErr
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBytes),
		}
	}

	if target != nil {
		limitedReader := io.LimitReader(resp.Body, 16*1024*1024)
		if err := json.NewDecoder(limitedReader).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w", err)
		}
	}

	return nil
}

// Health checks backend server connectivity
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}
	var res HealthResponse
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// SearchUsers searches platform users
func (c *Client) SearchUsers(ctx context.Context, query string) ([]User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/users/search?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[[]User]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListProjects gets all projects
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[[]Project]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListProjectsPaged gets paginated project list with filters
func (c *Client) ListProjectsPaged(ctx context.Context, page, limit int, status, q string) (*ApiResponse[[]Project], error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if status != "" {
		params.Set("status", status)
	}
	if q != "" {
		params.Set("q", q)
	}

	path := "/api/v1/projects"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var res ApiResponse[[]Project]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetProject fetches project details by UUID or slug
func (c *Client) GetProject(ctx context.Context, idOrSlug string) (*Project, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(idOrSlug), nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Project]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateProject creates a new project using CreateProjectRequest
func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/api/v1/projects", req)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Project]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// UpdateProject updates an existing project
func (c *Client) UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) (*Project, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(id), req)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Project]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// UpdateProjectStatus changes project lifecycle status
func (c *Client) UpdateProjectStatus(ctx context.Context, idOrSlug, status string) (*Project, error) {
	req, err := c.newRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(idOrSlug)+"/status", map[string]string{
		"status": status,
	})
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Project]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListMembers gets members of a project
func (c *Client) ListMembers(ctx context.Context, projectID string) ([]Member, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/members", nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[[]Member]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListMilestones retrieves all milestones for a project
func (c *Client) ListMilestones(ctx context.Context, projectID string) ([]Milestone, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/milestones", nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[[]Milestone]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// GetMilestone retrieves a specific milestone
func (c *Client) GetMilestone(ctx context.Context, projectID, milestoneID string) (*Milestone, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/milestones/"+url.PathEscape(milestoneID), nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Milestone]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateMilestone creates a new milestone
func (c *Client) CreateMilestone(ctx context.Context, projectID string, req CreateMilestoneRequest) (*Milestone, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/api/v1/projects/"+url.PathEscape(projectID)+"/milestones", req)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Milestone]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// UpdateMilestone updates a milestone
func (c *Client) UpdateMilestone(ctx context.Context, projectID, milestoneID string, req UpdateMilestoneRequest) (*Milestone, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(projectID)+"/milestones/"+url.PathEscape(milestoneID), req)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Milestone]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListRecords queries commits (progress records)
func (c *Client) ListRecords(ctx context.Context, projectID string, limit, offset int) ([]Record, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/records"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var res ApiResponse[[]Record]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListRecordsWithFilter queries commits with milestone and author filters
func (c *Client) ListRecordsWithFilter(ctx context.Context, projectID, milestoneID, authorID string, page, limit int) (*ApiResponse[[]Record], error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if milestoneID != "" {
		params.Set("milestoneId", milestoneID)
	}
	if authorID != "" {
		params.Set("authorId", authorID)
	}

	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/records"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var res ApiResponse[[]Record]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetRecord fetches a specific progress record
func (c *Client) GetRecord(ctx context.Context, projectID, recordID string) (*Record, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/records/"+url.PathEscape(recordID), nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[Record]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateRecord submits a new progress record
func (c *Client) CreateRecord(ctx context.Context, projectID string, req CreateRecordRequest) (*Record, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/api/v1/projects/"+url.PathEscape(projectID)+"/records", req)
	if err != nil {
		return nil, err
	}

	var res ApiResponse[Record]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// BindRecordMilestone associates or unbinds a record to a milestone
func (c *Client) BindRecordMilestone(ctx context.Context, projectID, recordID, milestoneID string) error {
	body := map[string]any{}
	if milestoneID == "" || milestoneID == "null" {
		body["milestoneId"] = nil
	} else {
		body["milestoneId"] = milestoneID
	}

	req, err := c.newRequest(ctx, http.MethodPatch, "/api/v1/records/"+url.PathEscape(recordID)+"/bind", body)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// BindRecord associates or unbinds a record to a milestone and returns updated Record
func (c *Client) BindRecord(ctx context.Context, recordID, milestoneID string) (*Record, error) {
	body := map[string]any{}
	if milestoneID == "" || milestoneID == "null" {
		body["milestoneId"] = nil
	} else {
		body["milestoneId"] = milestoneID
	}

	req, err := c.newRequest(ctx, http.MethodPatch, "/api/v1/records/"+url.PathEscape(recordID)+"/bind", body)
	if err != nil {
		return nil, err
	}

	var res ApiResponse[Record]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// DeleteRecord deletes a progress commit
func (c *Client) DeleteRecord(ctx context.Context, recordID string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/api/v1/records/"+url.PathEscape(recordID), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// GetConcept retrieves the concept doc blocks
func (c *Client) GetConcept(ctx context.Context, projectID string) (*ConceptDoc, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/concept", nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[ConceptDoc]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// GetConceptBlame retrieves the concept blocks with author attributions (Git Blame)
func (c *Client) GetConceptBlame(ctx context.Context, projectID, file string) ([]ConceptBlame, error) {
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/concept/blame"
	if file != "" {
		path += "?file=" + url.QueryEscape(file)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(req, &raw); err != nil {
		return nil, err
	}

	var blames []ConceptBlame
	if err := json.Unmarshal(raw.Data, &blames); err == nil && len(blames) > 0 {
		return blames, nil
	}

	var doc ConceptDoc
	if err := json.Unmarshal(raw.Data, &doc); err == nil {
		result := make([]ConceptBlame, len(doc.Blocks))
		for i, b := range doc.Blocks {
			result[i] = ConceptBlame{
				Id:          b.Id,
				Type:        b.Type,
				Text:        b.Text,
				Attribution: b.Attribution,
			}
		}
		return result, nil
	}

	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return nil, nil
	}
	return nil, fmt.Errorf("failed to decode concept blame payload: incompatible format")
}

// GetConceptDocBlame retrieves concept blame as ConceptDoc
func (c *Client) GetConceptDocBlame(ctx context.Context, projectID string) (*ConceptDoc, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/projects/"+url.PathEscape(projectID)+"/concept/blame", nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[ConceptDoc]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListConceptPRs queries concept change proposals (PRs)
func (c *Client) ListConceptPRs(ctx context.Context, projectID, status string) ([]ConceptPR, error) {
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/proposals"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[[]ConceptPR]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListProposals is an alias for ListConceptPRs
func (c *Client) ListProposals(ctx context.Context, projectID, status string) ([]Proposal, error) {
	return c.ListConceptPRs(ctx, projectID, status)
}

// GetConceptPR retrieves a specific concept proposal
func (c *Client) GetConceptPR(ctx context.Context, projectID, prID string) (*ConceptPR, error) {
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/proposals/" + url.PathEscape(prID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[ConceptPR]
	if err := c.do(req, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateConceptPR submits a new concept proposal (Git PR like)
func (c *Client) CreateConceptPR(ctx context.Context, projectID string, req CreateConceptPRRequest) (*ConceptPR, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/api/v1/projects/"+url.PathEscape(projectID)+"/proposals", req)
	if err != nil {
		return nil, err
	}
	var res ApiResponse[ConceptPR]
	if err := c.do(httpReq, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateProposal submits a new concept proposal with individual arguments
func (c *Client) CreateProposal(ctx context.Context, projectID, title, description string, patchData PatchData) (*Proposal, error) {
	return c.CreateConceptPR(ctx, projectID, CreateConceptPRRequest{
		Title:       title,
		Description: description,
		PatchData:   &patchData,
	})
}

// MergeProposal merges a proposal into concept doc
func (c *Client) MergeProposal(ctx context.Context, proposalID string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/proposals/"+url.PathEscape(proposalID)+"/merge", nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// CloseProposal closes/rejects a proposal
func (c *Client) CloseProposal(ctx context.Context, proposalID string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/proposals/"+url.PathEscape(proposalID)+"/close", nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}
