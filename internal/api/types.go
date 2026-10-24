// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"fmt"
	"time"
)

type APIError struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("server error HTTP %d: %s", e.StatusCode, e.Message)
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type ApiResponse[T any] struct {
	Data    T      `json:"data"`
	Meta    *Meta  `json:"meta,omitempty"`
	Message string `json:"message,omitempty"`
}

type User struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Name      *string `json:"name,omitempty"`
	AvatarUrl *string `json:"avatarUrl,omitempty"`
	Role      string  `json:"role,omitempty"`
}

func (u *User) DisplayName() string {
	if u == nil {
		return "Unknown"
	}
	if u.Name != nil && *u.Name != "" {
		return *u.Name
	}
	if u.Email != "" {
		return u.Email
	}
	if len(u.ID) > 8 {
		return u.ID[:8]
	}
	return u.ID
}

type Project struct {
	Id            string      `json:"id"`
	Title         string      `json:"title"`
	Slug          string      `json:"slug"`
	Description   *string     `json:"description,omitempty"`
	Visibility    string      `json:"visibility"`
	Status        string      `json:"status"` // IN_DEVELOPMENT, PUBLISHED, PLANNING, ARCHIVED
	Owner         *User       `json:"owner,omitempty"`
	MemberCount   int         `json:"memberCount,omitempty"`
	Members       []Member    `json:"members,omitempty"`
	Milestones    []Milestone `json:"milestones,omitempty"`
	RecentCommits []Record    `json:"recentRecords,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	ArchivedAt    *time.Time  `json:"archivedAt,omitempty"`
}

func (p *Project) ID() string {
	if p == nil {
		return ""
	}
	return p.Id
}

type Member struct {
	Id       string    `json:"id"`
	UserId   string    `json:"userId"`
	Role     string    `json:"role"` // OWNER, ADMIN, COLLABORATOR
	JoinedAt time.Time `json:"joinedAt"`
	User     *User     `json:"user,omitempty"`
}

type Milestone struct {
	Id          string    `json:"id"`
	ProjectId   string    `json:"projectId,omitempty"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // ROADMAP, FUTURE_MILESTONE
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"` // PLANNED, IN_PROGRESS, COMPLETED
	TargetDate  *string   `json:"targetDate,omitempty"`
	OrderIndex  int       `json:"orderIndex"`
	Records     []Record  `json:"records,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (m *Milestone) ID() string {
	if m == nil {
		return ""
	}
	return m.Id
}

type Record struct {
	Id          string     `json:"id"`
	ProjectId   string     `json:"projectId,omitempty"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	AuthorId    string     `json:"authorId,omitempty"`
	Author      *User      `json:"author,omitempty"`
	MilestoneId *string    `json:"milestoneId,omitempty"`
	Milestone   *Milestone `json:"milestone,omitempty"`
	CommittedAt time.Time  `json:"committedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (r *Record) ID() string {
	if r == nil {
		return ""
	}
	return r.Id
}

type ConceptBlock struct {
	Id          string             `json:"id"`
	Type        string             `json:"type"`
	Text        string             `json:"text"`
	Attribution *AttributionRecord `json:"attribution,omitempty"`
}

type AttributionRecord struct {
	Contributor   *User      `json:"contributor,omitempty"`
	ProposalId    *string    `json:"proposalId,omitempty"`
	ProposalTitle *string    `json:"proposalTitle,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type ConceptDoc struct {
	Version int            `json:"version"`
	Blocks  []ConceptBlock `json:"blocks"`
}

// NextBlockID generates a unique block ID (e.g. b-1, b-2) avoiding collisions with existing blocks
func (doc *ConceptDoc) NextBlockID() string {
	if doc == nil || len(doc.Blocks) == 0 {
		return "b-1"
	}
	existingIDs := make(map[string]bool, len(doc.Blocks))
	maxIdx := 0
	for _, b := range doc.Blocks {
		existingIDs[b.Id] = true
		var idx int
		if _, err := fmt.Sscanf(b.Id, "b-%d", &idx); err == nil && idx > maxIdx {
			maxIdx = idx
		}
	}

	candidate := maxIdx + 1
	if candidate < 1 {
		candidate = 1
	}
	newID := fmt.Sprintf("b-%d", candidate)
	for existingIDs[newID] {
		candidate++
		newID = fmt.Sprintf("b-%d", candidate)
	}
	return newID
}

type Proposal struct {
	Id          string     `json:"id"`
	ProjectId   string     `json:"projectId"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Status      string     `json:"status"` // OPEN, MERGED, CLOSED
	AuthorId    string     `json:"authorId"`
	Author      *User      `json:"author,omitempty"`
	PatchData   *PatchData `json:"patchData,omitempty"`
	MergedAt    *time.Time `json:"mergedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (p *Proposal) ID() string {
	if p == nil {
		return ""
	}
	return p.Id
}

type ConceptPR = Proposal

type PatchData struct {
	Blocks  []ConceptBlock `json:"blocks"`
	Summary string         `json:"summary,omitempty"`
}

type HealthResponse struct {
	Status    string  `json:"status"`
	Service   string  `json:"service"`
	Version   string  `json:"version"`
	Uptime    float64 `json:"uptime"`
	Timestamp string  `json:"timestamp"`
}

type CreateProjectRequest struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Status      string `json:"status,omitempty"`
}

type UpdateProjectRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Status      string `json:"status,omitempty"`
}

type CreateMilestoneRequest struct {
	Title       string  `json:"title"`
	Type        string  `json:"type"` // ROADMAP, FUTURE_MILESTONE
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"` // PLANNED, IN_PROGRESS, COMPLETED
	TargetDate  *string `json:"targetDate,omitempty"`
	OrderIndex  int     `json:"orderIndex,omitempty"`
}

type UpdateMilestoneRequest struct {
	Title       string  `json:"title,omitempty"`
	Type        string  `json:"type,omitempty"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
	TargetDate  *string `json:"targetDate,omitempty"`
	OrderIndex  *int    `json:"orderIndex,omitempty"`
}

type CreateRecordRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	MilestoneID string     `json:"milestoneId,omitempty"`
	CommittedAt *time.Time `json:"committedAt,omitempty"`
}

type CreateConceptPRRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	PatchData   *PatchData `json:"patchData,omitempty"`
}

type ConceptBlame struct {
	Id          string             `json:"id"`
	Type        string             `json:"type,omitempty"`
	Text        string             `json:"text,omitempty"`
	Attribution *AttributionRecord `json:"attribution,omitempty"`
}
