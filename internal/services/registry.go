package services

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/jwebster45206/story-engine/internal/config"
)

// ProviderInfo is the public projection of a provider (never includes API keys).
type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Vendor      string `json:"vendor"`
	Model       string `json:"model"`
}

// Registry maps provider names to initialized LLMService instances.
//
// A vendor is a wire protocol implemented in Go. A provider is a vendor plus a
// model declared in JSON. Adding a provider requires no Go change; adding a
// vendor means a new case in NewRegistry and a new adapter.
type Registry struct {
	services    map[string]LLMService
	info        map[string]ProviderInfo
	defaultName string
}

// NewRegistry builds one LLMService per configured provider.
func NewRegistry(cfg *config.Config, logger *slog.Logger) (*Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	r := &Registry{
		services:    make(map[string]LLMService, len(cfg.Providers)),
		info:        make(map[string]ProviderInfo, len(cfg.Providers)),
		defaultName: cfg.DefaultProvider,
	}
	for name, pc := range cfg.Providers {
		svc, err := newServiceForVendor(name, pc, logger)
		if err != nil {
			return nil, err
		}
		display := pc.DisplayName
		if display == "" {
			display = name
		}
		r.services[name] = svc
		r.info[name] = ProviderInfo{
			Name:        name,
			DisplayName: display,
			Vendor:      pc.Vendor,
			Model:       pc.Model,
		}
	}
	return r, nil
}

func newServiceForVendor(name string, pc *config.ProviderConfig, logger *slog.Logger) (LLMService, error) {
	switch strings.ToLower(pc.Vendor) {
	case config.VendorAnthropic:
		return NewAnthropicService(pc, logger), nil
	case config.VendorVenice:
		return NewVeniceService(pc, logger), nil
	default:
		return nil, fmt.Errorf("providers[%q]: unsupported vendor %q", name, pc.Vendor)
	}
}

// Get returns the LLMService for name. An empty name resolves to the default.
func (r *Registry) Get(name string) (LLMService, error) {
	if name == "" {
		name = r.defaultName
	}
	svc, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return svc, nil
}

// Names returns sorted provider names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Info returns the public projection for a provider.
func (r *Registry) Info(name string) (ProviderInfo, bool) {
	info, ok := r.info[name]
	return info, ok
}

// Default returns the default provider name.
func (r *Registry) Default() string {
	return r.defaultName
}
