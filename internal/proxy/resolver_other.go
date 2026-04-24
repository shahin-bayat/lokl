//go:build !darwin

package proxy

import "errors"

type resolverDir struct{ port int }

func newResolverDir(port int) *resolverDir { return &resolverDir{port: port} }

func (r *resolverDir) Write(parents []string) error {
	if len(parents) == 0 {
		return nil
	}
	return errors.New("wildcard DNS not supported on this platform yet (PR 2 adds Linux)")
}

func (r *resolverDir) Remove(parents []string) error { return nil }
func (r *resolverDir) FlushCache() error             { return nil }

// Missing reports all parents as missing on unsupported platforms so callers
// surface the gap instead of silently claiming DNS is wired up.
func (r *resolverDir) Missing(parents []string) []string { return parents }
