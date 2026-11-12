package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/store"
)

// SiteFor builds a Site from a stored upstream, decrypting its credential.
func SiteFor(u store.Upstream, box *crypto.Box) (Site, error) {
	s := Site{
		ID: u.ID, Name: u.Name, Type: u.Type, BaseURL: u.BaseURL, CredType: u.CredType,
		UserID: u.PlatformUserID, Config: u.AdapterConfig,
	}
	if len(u.Cred) == 0 {
		return s, nil
	}
	pt, err := box.Open(u.Cred)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(pt, &s.Cred); err != nil {
		return s, fmt.Errorf("upstream %s: bad credential payload: %w", u.Name, err)
	}
	return s, nil
}

// dbCredStore persists rotated credentials of one saved upstream.
type dbCredStore struct {
	st  *store.Store
	box *crypto.Box
	id  uint
}

// NewCredStore returns a CredStore backed by the upstreams table.
func NewCredStore(st *store.Store, box *crypto.Box, id uint) CredStore {
	return &dbCredStore{st: st, box: box, id: id}
}

func (d *dbCredStore) Load(ctx context.Context) (Credential, error) {
	var c Credential
	u, err := d.st.GetUpstream(ctx, d.id)
	if err != nil {
		return c, err
	}
	if len(u.Cred) == 0 {
		return c, nil
	}
	pt, err := d.box.Open(u.Cred)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(pt, &c); err != nil {
		return c, fmt.Errorf("upstream %s: bad credential payload: %w", u.Name, err)
	}
	return c, nil
}

func (d *dbCredStore) Save(ctx context.Context, c Credential) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	sealed, err := d.box.Seal(b)
	if err != nil {
		return err
	}
	return d.st.DB.WithContext(ctx).Model(&store.Upstream{}).Where("id = ?", d.id).
		Updates(map[string]any{"cred": sealed, "cred_updated_at": time.Now().Unix()}).Error
}
