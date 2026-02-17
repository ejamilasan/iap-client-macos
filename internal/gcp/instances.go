package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Instance represents a GCP Compute Engine instance
type Instance struct {
	Name        string `json:"name"`
	Zone        string `json:"zone"`
	Status      string `json:"status"`
	MachineType string `json:"machineType"`
	InternalIP  string `json:"internalIP"`
	ExternalIP  string `json:"externalIP"`
	Project     string `json:"project"`
	IsWindows   bool   `json:"isWindows"`
}

// InstanceService handles GCP Compute Engine instance operations
type InstanceService struct {
	tokenSource oauth2.TokenSource
}

// NewInstanceService creates a new instance service
func NewInstanceService(ts oauth2.TokenSource) *InstanceService {
	return &InstanceService{
		tokenSource: ts,
	}
}

// ListInstances returns all instances in a project
func (s *InstanceService) ListInstances(ctx context.Context, projectID string) ([]Instance, error) {
	if s.tokenSource == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	service, err := compute.NewService(ctx, option.WithTokenSource(s.tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create compute service: %w", err)
	}

	var instances []Instance

	// Use aggregated list to get instances from all zones
	req := service.Instances.AggregatedList(projectID)
	err = req.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		for zonePath, instanceList := range page.Items {
			if instanceList.Instances == nil {
				continue
			}

			// Extract zone name from path (e.g., "zones/us-central1-a")
			zone := zonePath
			if strings.HasPrefix(zonePath, "zones/") {
				zone = strings.TrimPrefix(zonePath, "zones/")
			}

			for _, inst := range instanceList.Instances {
				instance := Instance{
					Name:        inst.Name,
					Zone:        zone,
					Status:      inst.Status,
					MachineType: extractMachineType(inst.MachineType),
					Project:     projectID,
					IsWindows:   isWindowsInstance(inst),
				}

				// Get IP addresses
				if len(inst.NetworkInterfaces) > 0 {
					instance.InternalIP = inst.NetworkInterfaces[0].NetworkIP
					for _, accessConfig := range inst.NetworkInterfaces[0].AccessConfigs {
						if accessConfig.NatIP != "" {
							instance.ExternalIP = accessConfig.NatIP
							break
						}
					}
				}

				instances = append(instances, instance)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	// Sort by name
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

// ListWindowsInstances returns only Windows instances in a project
func (s *InstanceService) ListWindowsInstances(ctx context.Context, projectID string) ([]Instance, error) {
	instances, err := s.ListInstances(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var windowsInstances []Instance
	for _, inst := range instances {
		if inst.IsWindows {
			windowsInstances = append(windowsInstances, inst)
		}
	}

	return windowsInstances, nil
}

// GetInstance returns a specific instance
func (s *InstanceService) GetInstance(ctx context.Context, projectID, zone, instanceName string) (*Instance, error) {
	if s.tokenSource == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	service, err := compute.NewService(ctx, option.WithTokenSource(s.tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create compute service: %w", err)
	}

	inst, err := service.Instances.Get(projectID, zone, instanceName).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	instance := &Instance{
		Name:        inst.Name,
		Zone:        zone,
		Status:      inst.Status,
		MachineType: extractMachineType(inst.MachineType),
		Project:     projectID,
		IsWindows:   isWindowsInstance(inst),
	}

	// Get IP addresses
	if len(inst.NetworkInterfaces) > 0 {
		instance.InternalIP = inst.NetworkInterfaces[0].NetworkIP
		for _, accessConfig := range inst.NetworkInterfaces[0].AccessConfigs {
			if accessConfig.NatIP != "" {
				instance.ExternalIP = accessConfig.NatIP
				break
			}
		}
	}

	return instance, nil
}

// extractMachineType extracts the machine type name from the full URL
func extractMachineType(machineTypeURL string) string {
	parts := strings.Split(machineTypeURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return machineTypeURL
}

// isWindowsInstance checks if an instance is running Windows
func isWindowsInstance(inst *compute.Instance) bool {
	// Check disks for Windows licenses
	for _, disk := range inst.Disks {
		for _, license := range disk.Licenses {
			if strings.Contains(strings.ToLower(license), "windows") {
				return true
			}
		}
	}

	// Check metadata for Windows indicators
	if inst.Metadata != nil {
		for _, item := range inst.Metadata.Items {
			if item.Key == "windows-startup-script-ps1" ||
				item.Key == "windows-startup-script-cmd" ||
				item.Key == "windows-startup-script-bat" {
				return true
			}
		}
	}

	return false
}
