package app

import (
	"context"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// CreateResult is the answer of CreateFolder.
type CreateResult struct {
	ID      string   `json:"id"`
	Problem *Problem `json:"problem"`
}

// EditHostResult is the answer of EditHost.
type EditHostResult struct {
	// KeyCleared: the address changed, so the server's key is to be checked and trusted again.
	KeyCleared bool     `json:"keyCleared"`
	Problem    *Problem `json:"problem"`
}

// Editor serves the editing of the site manager: folders, names, places, colors, hosts. It only
// changes Vigie's data, never a server.
type Editor struct {
	ws *workspace.Workspace
}

// NewEditor returns the service over the workspace.
func NewEditor(ws *workspace.Workspace) *Editor {
	return &Editor{ws: ws}
}

// CreateFolder adds a folder under parentID, or at the root when it is empty.
func (e *Editor) CreateFolder(ctx context.Context, parentID, name string) CreateResult {
	parent := model.NoID
	if parentID != "" {
		id, err := uuid.Parse(parentID)
		if err != nil {
			return CreateResult{Problem: ProblemOf(model.ErrNotFound)}
		}
		parent = id
	}
	f, err := e.ws.CreateFolder(ctx, parent, name)
	if err != nil {
		return CreateResult{Problem: ProblemOf(err)}
	}
	return CreateResult{ID: f.ID.String()}
}

// Rename gives a folder, host or site a new name.
func (e *Editor) Rename(ctx context.Context, id, name string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.Rename(ctx, n, name))
}

// Move puts a folder or host in the folder folderID, or at the root when it is empty.
func (e *Editor) Move(ctx context.Context, id, folderID string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	folder := model.NoID
	if folderID != "" {
		if folder, err = uuid.Parse(folderID); err != nil {
			return ProblemOf(model.ErrNotFound)
		}
	}
	return ProblemOf(e.ws.Move(ctx, n, folder))
}

// SetColor gives a node one of the eight colors by name ("none": inherit its parent's).
func (e *Editor) SetColor(ctx context.Context, id, color string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	c, err := model.ParseColor(color)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.SetColor(ctx, n, c))
}

// SetNotes writes the Markdown notes of a node (ADR-0023), at most model.MaxNotes characters.
func (e *Editor) SetNotes(ctx context.Context, id, notes string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.SetNotes(ctx, n, notes))
}

// Delete removes a site, a host with its sites, or an empty folder from Vigie.
func (e *Editor) Delete(ctx context.Context, id string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.Delete(ctx, n))
}

// Host returns the details of a stored host, for its form.
func (e *Editor) Host(ctx context.Context, id string) (HostDetails, error) {
	n, err := e.node(ctx, id)
	if err != nil {
		return HostDetails{}, err
	}
	tree, err := e.ws.DB().LoadTree(ctx)
	if err != nil {
		return HostDetails{}, err
	}
	h, ok := tree.Host(n.ID)
	if !ok {
		return HostDetails{}, errNodeNotFound
	}
	return HostDetails{
		Name: h.Name, Alias: h.Alias, Hostname: h.Hostname, Port: h.Port, User: h.User, KeyPath: h.KeyPath,
		WPCLI: h.WPCLI, FolderID: idOrEmpty(h.FolderID), MaxConcurrent: h.MaxConcurrent,
	}, nil
}

// EditHost changes the name and connection of a stored host; its folder does not change here.
func (e *Editor) EditHost(ctx context.Context, id string, d HostDetails) EditHostResult {
	n, err := e.node(ctx, id)
	if err != nil {
		return EditHostResult{Problem: ProblemOf(err)}
	}
	if n.Kind != model.KindHost {
		return EditHostResult{Problem: ProblemOf(errNodeNotFound)}
	}
	h, err := d.host()
	if err != nil {
		return EditHostResult{Problem: ProblemOf(err)}
	}
	h.ID = n.ID
	cleared, err := e.ws.EditHost(ctx, h)
	return EditHostResult{KeyCleared: cleared, Problem: ProblemOf(err)}
}

// node finds what a stored ID is.
func (e *Editor) node(ctx context.Context, id string) (model.Node, error) {
	return findNode(ctx, e.ws, id)
}

func findNode(ctx context.Context, ws *workspace.Workspace, id string) (model.Node, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return model.Node{}, errNodeNotFound
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return model.Node{}, err
	}
	if _, ok := tree.Folder(parsed); ok {
		return model.Node{Kind: model.KindFolder, ID: parsed}, nil
	}
	if _, ok := tree.Host(parsed); ok {
		return model.Node{Kind: model.KindHost, ID: parsed}, nil
	}
	if _, ok := tree.Site(parsed); ok {
		return model.Node{Kind: model.KindSite, ID: parsed}, nil
	}
	return model.Node{}, errNodeNotFound
}
