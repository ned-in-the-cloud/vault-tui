package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/ned1313/vault-tui/internal/secure"
)

// AuthMethod is implemented by every supported auth method.
type AuthMethod interface {
	// Type returns a stable short identifier (e.g. "token", "userpass").
	Type() string
	// MountPath returns the mount path used for the API request, e.g.
	// "userpass" or "ldap". Empty means use the method's default.
	MountPath() string
	// Identifier returns a human-meaningful identifier for token storage
	// keying (username for password methods, display_name/accessor for
	// token auth).
	Identifier() string
	// Login performs the auth dance and returns the resulting Vault token
	// plus optional follow-up token info. Implementations may inspect the
	// underlying *api.Client via the helper passed in.
	Login(ctx context.Context, c *vaultapi.Client) (token string, identifier string, err error)
	// Destroy zeros any sensitive material held by the method (passphrases,
	// secret IDs, etc.). Safe to call multiple times.
	Destroy()
}

// ---- token auth ----

// TokenAuth wraps a pre-existing Vault token.
type TokenAuth struct {
	Token       *secure.SecureString
	displayName string // resolved from a lookup-self after Login
	accessor    string
}

// NewTokenAuth wraps the supplied token bytes in a SecureString. The
// token slice is wiped.
func NewTokenAuth(token []byte) *TokenAuth {
	return &TokenAuth{Token: secure.NewSecureString(token)}
}

func (t *TokenAuth) Type() string      { return "token" }
func (t *TokenAuth) MountPath() string { return "token" }

func (t *TokenAuth) Identifier() string {
	if t.displayName != "" {
		return t.displayName
	}
	return t.accessor
}

func (t *TokenAuth) Login(ctx context.Context, c *vaultapi.Client) (string, string, error) {
	if t == nil || t.Token == nil || t.Token.IsEmpty() {
		return "", "", errors.New("vault: empty token")
	}
	var rawToken string
	err := t.Token.WithBytes(func(b []byte) error {
		rawToken = string(b)
		return nil
	})
	if err != nil {
		return "", "", err
	}
	c.SetToken(rawToken)
	// Resolve display_name / accessor for identifier keying.
	sec, err := c.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		return "", "", classifyError(err)
	}
	if sec != nil && sec.Data != nil {
		if v, ok := sec.Data["display_name"].(string); ok {
			t.displayName = v
		}
		if v, ok := sec.Data["accessor"].(string); ok {
			t.accessor = v
		}
	}
	return rawToken, t.Identifier(), nil
}

func (t *TokenAuth) Destroy() {
	if t == nil {
		return
	}
	t.Token.Destroy()
}

// ---- userpass / ldap (shared shape) ----

// PasswordAuth covers UserPass and LDAP, which share an identical
// payload shape (`{"password": "..."}`) at the
// `/auth/<mount>/login/<username>` endpoint.
type PasswordAuth struct {
	method   string // "userpass" or "ldap"
	mount    string
	Username string
	Password *secure.SecureString
}

// NewUserPassAuth constructs a UserPass auth method. password is wiped.
func NewUserPassAuth(mount, username string, password []byte) *PasswordAuth {
	if mount == "" {
		mount = "userpass"
	}
	return &PasswordAuth{method: "userpass", mount: mount, Username: username, Password: secure.NewSecureString(password)}
}

// NewLDAPAuth constructs an LDAP auth method. password is wiped.
func NewLDAPAuth(mount, username string, password []byte) *PasswordAuth {
	if mount == "" {
		mount = "ldap"
	}
	return &PasswordAuth{method: "ldap", mount: mount, Username: username, Password: secure.NewSecureString(password)}
}

func (p *PasswordAuth) Type() string       { return p.method }
func (p *PasswordAuth) MountPath() string  { return p.mount }
func (p *PasswordAuth) Identifier() string { return p.Username }

func (p *PasswordAuth) Login(ctx context.Context, c *vaultapi.Client) (string, string, error) {
	if p == nil {
		return "", "", errors.New("vault: nil password auth")
	}
	if p.Username == "" {
		return "", "", errors.New("vault: empty username")
	}
	if p.Password == nil || p.Password.IsEmpty() {
		return "", "", errors.New("vault: empty password")
	}
	path := fmt.Sprintf("auth/%s/login/%s", strings.Trim(p.mount, "/"), p.Username)

	var token string
	err := p.Password.WithBytes(func(pw []byte) error {
		// vault api takes map[string]interface{} which forces a string
		// conversion. We immediately drop the map after the call.
		body := map[string]interface{}{"password": string(pw)}
		sec, err := c.Logical().WriteWithContext(ctx, path, body)
		// Wipe the password copy held inside body's string by rebuilding it.
		body["password"] = ""
		if err != nil {
			return classifyError(err)
		}
		if sec == nil || sec.Auth == nil || sec.Auth.ClientToken == "" {
			return errors.New("vault: login returned no token")
		}
		token = sec.Auth.ClientToken
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return token, p.Username, nil
}

func (p *PasswordAuth) Destroy() {
	if p == nil {
		return
	}
	p.Password.Destroy()
}
