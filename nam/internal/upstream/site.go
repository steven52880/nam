package upstream

import (
	"encoding/json"
	"fmt"

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
