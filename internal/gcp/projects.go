package gcp

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/oauth2"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
)

// Project represents a GCP project
type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number int64  `json:"number"`
}

// ProjectService handles GCP project operations
type ProjectService struct {
	tokenSource oauth2.TokenSource
}

// NewProjectService creates a new project service
func NewProjectService(ts oauth2.TokenSource) *ProjectService {
	return &ProjectService{
		tokenSource: ts,
	}
}

// ListProjects returns all accessible GCP projects
func (s *ProjectService) ListProjects(ctx context.Context) ([]Project, error) {
	if s.tokenSource == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	service, err := cloudresourcemanager.NewService(ctx, option.WithTokenSource(s.tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager service: %w", err)
	}

	var projects []Project
	pageToken := ""

	for {
		req := service.Projects.List()
		if pageToken != "" {
			req.PageToken(pageToken)
		}

		// Filter for active projects only
		req.Filter("lifecycleState:ACTIVE")

		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		for _, p := range resp.Projects {
			projects = append(projects, Project{
				ID:     p.ProjectId,
				Name:   p.Name,
				Number: p.ProjectNumber,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	// Sort by name
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

// GetProject returns a specific project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*Project, error) {
	if s.tokenSource == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	service, err := cloudresourcemanager.NewService(ctx, option.WithTokenSource(s.tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager service: %w", err)
	}

	p, err := service.Projects.Get(projectID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &Project{
		ID:     p.ProjectId,
		Name:   p.Name,
		Number: p.ProjectNumber,
	}, nil
}
