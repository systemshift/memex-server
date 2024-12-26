package repository

import (
	"fmt"
	"memex/internal/memex/core"
	"memex/pkg/types"
)

// Convert core.Node to types.Node
func convertNodeToTypes(node *core.Node) *types.Node {
	if node == nil {
		return nil
	}
	return &types.Node{
		ID:       node.ID,
		Type:     node.Type,
		Content:  node.Content,
		Meta:     node.Meta,
		Created:  node.Created,
		Modified: node.Modified,
	}
}

// Convert core.Link to types.Link
func convertLinkToTypes(link *core.Link) *types.Link {
	if link == nil {
		return nil
	}
	return &types.Link{
		Source:   link.Source,
		Target:   link.Target,
		Type:     link.Type,
		Meta:     link.Meta,
		Created:  link.Created,
		Modified: link.Modified,
	}
}

// Convert core.Command to types.Command
func convertCommandToTypes(cmd core.Command) types.Command {
	return types.Command{
		Name:        cmd.Name,
		Description: cmd.Description,
		Usage:       cmd.Usage,
		Args:        cmd.Args,
	}
}

// ModuleAdapter converts core.Module to types.Module
type ModuleAdapter struct {
	core.Module
}

// NewModuleAdapter creates a new module adapter
func NewModuleAdapter(module interface{}) types.Module {
	switch m := module.(type) {
	case core.Module:
		return &ModuleAdapter{Module: m}
	case types.Module:
		return m
	default:
		panic(fmt.Sprintf("unsupported module type: %T", module))
	}
}

// NewReverseModuleAdapter creates a new reverse module adapter
func NewReverseModuleAdapter(module types.Module) core.Module {
	return &reverseModuleAdapter{Module: module}
}

func (m *ModuleAdapter) Commands() []types.Command {
	coreCmds := m.Module.Commands()
	typeCmds := make([]types.Command, len(coreCmds))
	for i, cmd := range coreCmds {
		typeCmds[i] = convertCommandToTypes(cmd)
	}
	return typeCmds
}

func (m *ModuleAdapter) Init(repo types.Repository) error {
	// Convert types.Repository to core.Repository if needed
	if adapter, ok := repo.(*repositoryAdapter); ok {
		return m.Module.Init(adapter.Repository)
	}
	// Create a new adapter that implements core.Repository
	return m.Module.Init(&reverseRepositoryAdapter{repo})
}

// Convert types.Repository to core.Repository
type reverseRepositoryAdapter struct {
	types.Repository
}

func (r *reverseRepositoryAdapter) GetNode(id string) (*core.Node, error) {
	node, err := r.Repository.GetNode(id)
	if err != nil {
		return nil, err
	}
	return &core.Node{
		ID:       node.ID,
		Type:     node.Type,
		Content:  node.Content,
		Meta:     node.Meta,
		Created:  node.Created,
		Modified: node.Modified,
	}, nil
}

func (r *reverseRepositoryAdapter) GetLinks(nodeID string) ([]*core.Link, error) {
	links, err := r.Repository.GetLinks(nodeID)
	if err != nil {
		return nil, err
	}
	coreLinks := make([]*core.Link, len(links))
	for i, link := range links {
		coreLinks[i] = &core.Link{
			Source:   link.Source,
			Target:   link.Target,
			Type:     link.Type,
			Meta:     link.Meta,
			Created:  link.Created,
			Modified: link.Modified,
		}
	}
	return coreLinks, nil
}

func (r *reverseRepositoryAdapter) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
	nodes, err := r.Repository.QueryNodesByModule(moduleID)
	if err != nil {
		return nil, err
	}
	coreNodes := make([]*core.Node, len(nodes))
	for i, node := range nodes {
		coreNodes[i] = &core.Node{
			ID:       node.ID,
			Type:     node.Type,
			Content:  node.Content,
			Meta:     node.Meta,
			Created:  node.Created,
			Modified: node.Modified,
		}
	}
	return coreNodes, nil
}

func (r *reverseRepositoryAdapter) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
	links, err := r.Repository.QueryLinksByModule(moduleID)
	if err != nil {
		return nil, err
	}
	coreLinks := make([]*core.Link, len(links))
	for i, link := range links {
		coreLinks[i] = &core.Link{
			Source:   link.Source,
			Target:   link.Target,
			Type:     link.Type,
			Meta:     link.Meta,
			Created:  link.Created,
			Modified: link.Modified,
		}
	}
	return coreLinks, nil
}

func (r *reverseRepositoryAdapter) ListModules() []core.Module {
	typeMods := r.Repository.ListModules()
	coreMods := make([]core.Module, len(typeMods))
	for i, mod := range typeMods {
		coreMods[i] = &reverseModuleAdapter{mod}
	}
	return coreMods
}

func (r *reverseRepositoryAdapter) GetModule(id string) (core.Module, bool) {
	mod, exists := r.Repository.GetModule(id)
	if !exists {
		return nil, false
	}
	return &reverseModuleAdapter{Module: mod}, true
}

func (r *reverseRepositoryAdapter) RegisterModule(module core.Module) error {
	// Convert back to types.Module if needed
	if adapter, ok := module.(*reverseModuleAdapter); ok {
		return r.Repository.RegisterModule(adapter.Module)
	}
	// Create a new adapter that implements types.Module
	return r.Repository.RegisterModule(&ModuleAdapter{Module: module})
}

// Convert core.Repository to types.Repository
type repositoryAdapter struct {
	*Repository
}

func (r *repositoryAdapter) GetNode(id string) (*types.Node, error) {
	node, err := r.Repository.GetNode(id)
	if err != nil {
		return nil, err
	}
	return convertNodeToTypes(node), nil
}

func (r *repositoryAdapter) GetLinks(nodeID string) ([]*types.Link, error) {
	links, err := r.Repository.GetLinks(nodeID)
	if err != nil {
		return nil, err
	}
	typeLinks := make([]*types.Link, len(links))
	for i, link := range links {
		typeLinks[i] = convertLinkToTypes(link)
	}
	return typeLinks, nil
}

func (r *repositoryAdapter) QueryNodesByModule(moduleID string) ([]*types.Node, error) {
	nodes, err := r.Repository.QueryNodesByModule(moduleID)
	if err != nil {
		return nil, err
	}
	typeNodes := make([]*types.Node, len(nodes))
	for i, node := range nodes {
		typeNodes[i] = convertNodeToTypes(node)
	}
	return typeNodes, nil
}

func (r *repositoryAdapter) QueryLinksByModule(moduleID string) ([]*types.Link, error) {
	links, err := r.Repository.QueryLinksByModule(moduleID)
	if err != nil {
		return nil, err
	}
	typeLinks := make([]*types.Link, len(links))
	for i, link := range links {
		typeLinks[i] = convertLinkToTypes(link)
	}
	return typeLinks, nil
}

func (r *repositoryAdapter) ListModules() []types.Module {
	coreMods := r.Repository.ListModules()
	typeMods := make([]types.Module, len(coreMods))
	for i, mod := range coreMods {
		typeMods[i] = &ModuleAdapter{mod}
	}
	return typeMods
}

func (r *repositoryAdapter) GetModule(id string) (types.Module, bool) {
	mod, exists := r.Repository.GetModule(id)
	if !exists {
		return nil, false
	}
	return &ModuleAdapter{mod}, true
}

func (r *repositoryAdapter) RegisterModule(module types.Module) error {
	// Convert back to core.Module if needed
	if adapter, ok := module.(*ModuleAdapter); ok {
		return r.Repository.RegisterModule(adapter.Module)
	}
	// Create a new adapter that implements core.Module
	return r.Repository.RegisterModule(&reverseModuleAdapter{module})
}

// Convert types.Module to core.Module
type reverseModuleAdapter struct {
	types.Module
}

func (m *reverseModuleAdapter) Commands() []core.Command {
	typeCmds := m.Module.Commands()
	coreCmds := make([]core.Command, len(typeCmds))
	for i, cmd := range typeCmds {
		coreCmds[i] = core.Command{
			Name:        cmd.Name,
			Description: cmd.Description,
			Usage:       cmd.Usage,
			Args:        cmd.Args,
		}
	}
	return coreCmds
}

func (m *reverseModuleAdapter) Init(repo core.Repository) error {
	// Convert core.Repository to types.Repository
	var typesRepo types.Repository
	if r, ok := repo.(*Repository); ok {
		typesRepo = NewRepositoryAdapter(r)
	} else {
		typesRepo = &repositoryAdapter{repo.(*Repository)}
	}
	return m.Module.Init(typesRepo)
}

// NewRepositoryAdapter creates a new repository adapter
func NewRepositoryAdapter(repo *Repository) types.ModuleRepository {
	return &repositoryAdapter{repo}
}

func (r *repositoryAdapter) GetLoader() types.ModuleLoader {
	return r.Repository.loader
}

func (r *repositoryAdapter) GetDiscovery() types.ModuleDiscovery {
	return r.Repository.discovery
}
