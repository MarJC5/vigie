package workspace

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
)

var (
	// ErrNotAFolder is returned when an import is asked to add into a host or a site.
	ErrNotAFolder = errors.New("not a folder")
	// ErrKeyChanged is returned when a key confirmed in a preview can no longer be trusted: Vigie
	// trusts another key for that server since.
	ErrKeyChanged = errors.New("another key is trusted for this server since the preview")
)

// ImportStatus is what an import does with a folder, host or site of the file.
type ImportStatus string

const (
	ImportAdded   ImportStatus = "added"
	ImportPresent ImportStatus = "present" // already there: left as it is
	ImportRefused ImportStatus = "refused"
)

// ImportReason tells why a node is present or refused, or that it was added under another name.
type ImportReason string

const (
	ReasonSameID         ImportReason = "same-id"
	ReasonSameName       ImportReason = "same-name"       // a folder: the one of that name is used
	ReasonSameConnection ImportReason = "same-connection" // a host: same alias, or hostname, port and user
	ReasonSamePath       ImportReason = "same-path"       // a site: same host and path
	ReasonRenamed        ImportReason = "renamed"         // added, its name taken by a sibling
	ReasonInvalid        ImportReason = "invalid"
	ReasonParentRefused  ImportReason = "parent-refused"
)

// ImportEntry is a folder, host or site of the file and what the import does with it.
type ImportEntry struct {
	Kind   model.NodeKind
	Name   string // in the file
	Path   string // once imported: the node added, or the one already there
	Status ImportStatus
	Reason ImportReason // empty when added as it is
	Detail string       // for a node refused, why
}

// KeyRefusal tells why a key of the file cannot be trusted; empty when the user can confirm it.
type KeyRefusal string

const (
	KeyInvalid      KeyRefusal = "invalid"
	KeyOtherServer  KeyRefusal = "other-server"  // its name is not the host's
	KeyUnresolved   KeyRefusal = "unresolved"    // the host's alias does not resolve here
	KeyOnePerHost   KeyRefusal = "one-per-host"  // the host has one already
	KeyOtherTrusted KeyRefusal = "other-trusted" // Vigie trusts another key for that server
)

// ImportKey is a key of the file for a host the import adds, shown with its fingerprint for the
// user to confirm.
type ImportKey struct {
	HostID      model.ID // the host as added
	Host        string   // its path
	Name        string   // the server's name in known_hosts
	Key         sshx.HostKey
	Fingerprint string
	Refused     KeyRefusal
}

// ImportPlan is what an import would do, shown before it does it.
type ImportPlan struct {
	Into                       string // the folder it adds into; "" for the top
	Entries                    []ImportEntry
	Keys                       []ImportKey
	Ignores, Scans, Operations int // added with the sites added
	batch                      store.ImportBatch
}

// Added counts the folders, hosts and sites the import adds.
func (p ImportPlan) Added() int {
	n := 0
	for _, e := range p.Entries {
		if e.Status == ImportAdded {
			n++
		}
	}
	return n
}

// PlanImport reads what importing f into the folder at path ("" for the top) would do: what is
// new is added, what is already there is left as it is, and nothing stored changes. The file's
// IDs are kept, a new one given only when another node has it.
func (w *Workspace) PlanImport(ctx context.Context, f exchange.File, into string) (ImportPlan, error) {
	if err := f.Check(); err != nil {
		return ImportPlan{}, err
	}
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return ImportPlan{}, err
	}
	target := model.Root
	if into != "" {
		if target, err = tree.Resolve(into); err != nil {
			return ImportPlan{}, err
		}
		if target.Kind != model.KindFolder {
			return ImportPlan{}, fmt.Errorf("%q: %w", into, ErrNotAFolder)
		}
	}
	p := planner{w: w, tree: tree, file: f, plan: ImportPlan{Into: tree.PathOf(target)},
		folders: map[string]mapped{}, hosts: map[string]mapped{}, sites: map[string]model.ID{},
		added: map[model.ID]bool{}, next: map[slot]int{}, trusting: map[string]string{}}
	p.existing(target.ID)
	for _, x := range f.Folders {
		p.folder(x)
	}
	for _, x := range f.Hosts {
		if err := p.host(ctx, x); err != nil {
			return ImportPlan{}, err
		}
	}
	for _, x := range f.Sites {
		p.site(x)
	}
	p.history()
	return p.plan, nil
}

// mapped is where a node of the file goes: the node added or the one already there, or nowhere.
type mapped struct {
	id      model.ID
	path    string // where it is, or would have been when refused
	refused bool
}

// slot is a list of siblings: the folders or hosts of a folder, or the sites of a host.
type slot struct {
	kind   model.NodeKind
	parent model.ID
}

type planner struct {
	w        *Workspace
	tree     *model.Tree
	file     exchange.File
	plan     ImportPlan
	into     model.ID
	folders  map[string]mapped   // by the file's ID
	hosts    map[string]mapped   // by the file's ID
	sites    map[string]model.ID // the sites added, by the file's ID
	added    map[model.ID]bool
	stored   []model.Host
	next     map[slot]int      // the position after the last stored sibling
	trusting map[string]string // the fingerprints offered in this import, by server name
}

// existing reads what is stored before anything is planned: the hosts, and where each list of
// siblings ends, so what is added comes after it.
func (p *planner) existing(into model.ID) {
	p.into = into
	p.tree.Walk(func(n model.Node, _ int) {
		var s slot
		var position int
		switch n.Kind {
		case model.KindFolder:
			f, _ := p.tree.Folder(n.ID)
			s, position = slot{model.KindFolder, f.ParentID}, f.Position
		case model.KindHost:
			h, _ := p.tree.Host(n.ID)
			p.stored = append(p.stored, h)
			s, position = slot{model.KindHost, h.FolderID}, h.Position
		case model.KindSite:
			site, _ := p.tree.Site(n.ID)
			s, position = slot{model.KindSite, site.HostID}, site.Position
		}
		p.next[s] = max(p.next[s], position+1)
	})
}

// position places a node added: after the stored siblings when its parent was already there,
// as in the file under a parent added.
func (p *planner) position(kind model.NodeKind, parent model.ID, position int) int {
	if p.added[parent] {
		return position
	}
	return p.next[slot{kind, parent}] + position
}

// newID keeps the file's ID unless a stored or planned node has it.
func (p *planner) newID(id model.ID) model.ID {
	_, folder := p.tree.Folder(id)
	_, host := p.tree.Host(id)
	_, site := p.tree.Site(id)
	if id == model.NoID || folder || host || site {
		return model.NewID()
	}
	return id
}

func (p *planner) entry(kind model.NodeKind, name, path string, status ImportStatus, reason ImportReason, detail string) {
	p.plan.Entries = append(p.plan.Entries, ImportEntry{Kind: kind, Name: name, Path: path, Status: status, Reason: reason, Detail: detail})
}

// add tries name, then "name (2)", "name (3)"... while a sibling has it; it returns the reason
// to show.
func add(name string, try func(name string) error) (ImportReason, error) {
	for n := 1; ; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s (%d)", name, n)
		}
		err := try(candidate)
		if !errors.Is(err, model.ErrDuplicateName) {
			if err == nil && n > 1 {
				return ReasonRenamed, nil
			}
			return "", err
		}
	}
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// folder maps a folder of the file, its parents first.
func (p *planner) folder(x exchange.Folder) mapped {
	if m, ok := p.folders[x.ID]; ok {
		return m
	}
	parent := mapped{id: p.into, path: p.plan.Into}
	if x.Parent != "" {
		for _, f := range p.file.Folders {
			if f.ID == x.Parent {
				parent = p.folder(f)
			}
		}
	}
	m := p.placeFolder(x, parent)
	p.folders[x.ID] = m
	return m
}

func (p *planner) placeFolder(x exchange.Folder, parent mapped) mapped {
	path := joinPath(parent.path, x.Name)
	refused := func(reason ImportReason, detail string) mapped {
		p.entry(model.KindFolder, x.Name, path, ImportRefused, reason, detail)
		return mapped{path: path, refused: true}
	}
	if parent.refused {
		return refused(ReasonParentRefused, "")
	}
	f := x.Model()
	folderPath := func(id model.ID) string { return p.tree.PathOf(model.Node{Kind: model.KindFolder, ID: id}) }
	if _, ok := p.tree.Folder(f.ID); ok {
		p.entry(model.KindFolder, x.Name, folderPath(f.ID), ImportPresent, ReasonSameID, "")
		return mapped{id: f.ID, path: folderPath(f.ID)}
	}
	if err := model.ValidateName(x.Name); err != nil {
		return refused(ReasonInvalid, err.Error())
	}
	if n, err := p.tree.Resolve(path); err == nil && n.Kind == model.KindFolder && !p.added[n.ID] {
		p.entry(model.KindFolder, x.Name, folderPath(n.ID), ImportPresent, ReasonSameName, "")
		return mapped{id: n.ID, path: folderPath(n.ID)}
	}
	f.ID, f.ParentID = p.newID(f.ID), parent.id
	f.Position = p.position(model.KindFolder, parent.id, f.Position)
	reason, err := add(x.Name, func(name string) error {
		f.Name = name
		return p.tree.AddFolder(f)
	})
	if err != nil {
		return refused(ReasonInvalid, err.Error())
	}
	p.added[f.ID] = true
	p.plan.batch.Folders = append(p.plan.batch.Folders, f)
	p.entry(model.KindFolder, x.Name, folderPath(f.ID), ImportAdded, reason, "")
	return mapped{id: f.ID, path: folderPath(f.ID)}
}

// sameConnection tells whether two hosts reach the same account: the same alias, or the same
// hostname, port and user.
func sameConnection(a, b model.Host) bool {
	if a.Alias != "" || b.Alias != "" {
		return a.Alias == b.Alias
	}
	return strings.EqualFold(a.Hostname, b.Hostname) && a.EffectivePort() == b.EffectivePort() && a.User == b.User
}

func (p *planner) host(ctx context.Context, x exchange.Host) error {
	parent := mapped{id: p.into, path: p.plan.Into}
	if x.Folder != "" {
		parent = p.folders[x.Folder]
	}
	path := joinPath(parent.path, x.Name)
	h := x.Model()
	hostPath := func(id model.ID) string { return p.tree.PathOf(model.Node{Kind: model.KindHost, ID: id}) }
	place := func(status ImportStatus, reason ImportReason, detail string, m mapped) {
		p.entry(model.KindHost, x.Name, m.path, status, reason, detail)
		p.hosts[x.ID] = m
	}
	if parent.refused {
		place(ImportRefused, ReasonParentRefused, "", mapped{path: path, refused: true})
		return nil
	}
	if p.hasHost(h.ID) {
		place(ImportPresent, ReasonSameID, "", mapped{id: h.ID, path: hostPath(h.ID)})
		return nil
	}
	for _, stored := range p.stored {
		if sameConnection(stored, h) {
			place(ImportPresent, ReasonSameConnection, "", mapped{id: stored.ID, path: hostPath(stored.ID)})
			return nil
		}
	}
	if err := model.ValidateName(x.Name); err != nil {
		place(ImportRefused, ReasonInvalid, err.Error(), mapped{path: path, refused: true})
		return nil
	}
	h.ID, h.FolderID = p.newID(h.ID), parent.id
	h.Position = p.position(model.KindHost, parent.id, h.Position)
	reason, err := add(x.Name, func(name string) error {
		h.Name = name
		return p.tree.AddHost(h)
	})
	if err != nil {
		place(ImportRefused, ReasonInvalid, err.Error(), mapped{path: path, refused: true})
		return nil
	}
	p.added[h.ID] = true
	place(ImportAdded, reason, "", mapped{id: h.ID, path: hostPath(h.ID)})
	p.plan.batch.Hosts = append(p.plan.batch.Hosts, h)
	return p.keys(ctx, h, hostPath(h.ID), x.TrustedKeys)
}

func (p *planner) hasHost(id model.ID) bool {
	_, ok := p.tree.Host(id)
	return ok && !p.added[id]
}

// keys offers the keys of a host added: the first of its server's name, when Vigie trusts no
// other key for it.
func (p *planner) keys(ctx context.Context, h model.Host, path string, keys []exchange.HostKey) error {
	if len(keys) == 0 {
		return nil
	}
	name, resolved := sshx.KnownHostsName(h.Hostname, h.EffectivePort()), true
	if h.Alias != "" {
		resolved = false
		if p.w.deps.Resolver != nil {
			if r, err := p.w.deps.Resolver.Resolve(ctx, h.Alias); err == nil {
				name, resolved = r.KnownHostsName(), true
			}
		}
	}
	offered := false
	for _, k := range keys {
		key := ImportKey{HostID: h.ID, Host: path, Name: k.Name, Key: sshx.HostKey{Type: k.Type, Key: k.Key}}
		fp, err := key.Key.Fingerprint()
		key.Fingerprint = fp
		switch {
		case err != nil:
			key.Refused = KeyInvalid
		case !resolved:
			key.Refused = KeyUnresolved
		case k.Name != name:
			key.Refused = KeyOtherServer
		case offered:
			key.Refused = KeyOnePerHost
		default:
			other, err := p.otherTrusted(name, fp)
			if err != nil {
				return err
			}
			if other {
				key.Refused = KeyOtherTrusted
			} else {
				offered = true
				p.trusting[name] = fp
			}
		}
		p.plan.Keys = append(p.plan.Keys, key)
	}
	return nil
}

// otherTrusted tells whether Vigie trusts, or this import offers, another key for the server.
func (p *planner) otherTrusted(name, fingerprint string) (bool, error) {
	if fp, ok := p.trusting[name]; ok && fp != fingerprint {
		return true, nil
	}
	return p.w.trustsOther(name, fingerprint)
}

// trustsOther tells whether Vigie's known_hosts has a key other than the one given for name.
func (w *Workspace) trustsOther(name, fingerprint string) (bool, error) {
	if w.deps.KnownHosts == "" {
		return false, nil
	}
	trusted, err := sshx.TrustedKeys(w.deps.KnownHosts, name)
	if err != nil {
		return false, err
	}
	for _, k := range trusted {
		if fp, err := k.Fingerprint(); err == nil && fp != fingerprint {
			return true, nil
		}
	}
	return false, nil
}

func (p *planner) site(x exchange.Site) {
	host := p.hosts[x.Host]
	path := joinPath(host.path, x.Name)
	s := x.Model()
	sitePath := func(id model.ID) string { return p.tree.PathOf(model.Node{Kind: model.KindSite, ID: id}) }
	if host.refused {
		p.entry(model.KindSite, x.Name, path, ImportRefused, ReasonParentRefused, "")
		return
	}
	if stored, ok := p.tree.Site(s.ID); ok && !p.added[stored.ID] {
		p.entry(model.KindSite, x.Name, sitePath(s.ID), ImportPresent, ReasonSameID, "")
		return
	}
	if !p.added[host.id] {
		for _, stored := range p.tree.SitesUnder(model.Node{Kind: model.KindHost, ID: host.id}) {
			if !p.added[stored.ID] && samePath(stored.Path, s.Path) {
				p.entry(model.KindSite, x.Name, sitePath(stored.ID), ImportPresent, ReasonSamePath, "")
				return
			}
		}
	}
	s.ID, s.HostID = p.newID(s.ID), host.id
	s.Position = p.position(model.KindSite, host.id, s.Position)
	if err := model.ValidateName(x.Name); err != nil {
		p.entry(model.KindSite, x.Name, path, ImportRefused, ReasonInvalid, err.Error())
		return
	}
	reason, err := add(x.Name, func(name string) error {
		s.Name = name
		return p.tree.AddSite(s)
	})
	if err != nil {
		p.entry(model.KindSite, x.Name, path, ImportRefused, ReasonInvalid, err.Error())
		return
	}
	p.added[s.ID] = true
	p.sites[x.ID] = s.ID
	p.plan.batch.Sites = append(p.plan.batch.Sites, s)
	p.entry(model.KindSite, x.Name, sitePath(s.ID), ImportAdded, reason, "")
}

// samePath compares two WordPress roots, a trailing slash aside.
func samePath(a, b string) bool {
	return strings.TrimSuffix(a, "/") == strings.TrimSuffix(b, "/")
}

// history keeps the ignores, scans and operations of the sites added; the others keep theirs.
func (p *planner) history() {
	for _, x := range p.file.Ignores {
		if id, ok := p.sites[x.Site]; ok {
			p.plan.batch.Ignores = append(p.plan.batch.Ignores, x.Model(id))
		}
	}
	if p.file.History != nil {
		for _, x := range p.file.History.Scans {
			if id, ok := p.sites[x.Site]; ok {
				p.plan.batch.Scans = append(p.plan.batch.Scans, x.Model(id))
			}
		}
		for _, x := range p.file.History.Operations {
			if id, ok := p.sites[x.Site]; ok {
				p.plan.batch.Operations = append(p.plan.batch.Operations, x.Model(id))
			}
		}
	}
	// Scans in the order they were made: the latest complete one is the site's state.
	slices.SortStableFunc(p.plan.batch.Scans, func(a, b model.Scan) int { return cmp.Compare(a.Started.UnixNano(), b.Started.UnixNano()) })
	p.plan.Ignores, p.plan.Scans, p.plan.Operations = len(p.plan.batch.Ignores), len(p.plan.batch.Scans), len(p.plan.batch.Operations)
}

// Import does what a plan said: the keys confirmed (by their host's ID) go into Vigie's
// known_hosts, then everything is added in one transaction. A host whose key was not confirmed
// asks at its first connection, as a new host does. Nothing stored changes: a plan made before
// another change that clashes with it is refused whole.
func (w *Workspace) Import(ctx context.Context, plan ImportPlan, confirmed []model.ID) error {
	batch := plan.batch
	batch.Hosts = slices.Clone(batch.Hosts)
	for _, k := range plan.Keys {
		if k.Refused != "" || !slices.Contains(confirmed, k.HostID) {
			continue
		}
		other, err := w.trustsOther(k.Name, k.Fingerprint)
		if err != nil {
			return err
		}
		if other {
			return fmt.Errorf("%s: %w", k.Name, ErrKeyChanged)
		}
		if err := w.Trust(k.Name, k.Key); err != nil {
			return err
		}
		for i := range batch.Hosts {
			if batch.Hosts[i].ID == k.HostID {
				batch.Hosts[i].HostKeyFingerprint = k.Fingerprint
			}
		}
	}
	return w.db.Import(ctx, batch)
}
