//go:build !windows

package sshtunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// agentAuth offers the keys the reader has already unlocked. Everywhere but Windows the agent is
// a Unix socket whose path is in the environment, which is also how its absence is reported: an
// empty variable means no agent was ever started for this session.
//
// The socket has to stay open for the whole handshake, since that is when the signing happens, so
// it is handed back to be closed by the caller rather than closed here.
func agentAuth() (ssh.AuthMethod, io.Closer, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, nil, fmt.Errorf(
			"%w: no SSH agent — SSH_AUTH_SOCK is not set",
			domain.ErrTunnelCredentials,
		)
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("%w: cannot reach the SSH agent", domain.ErrTunnelCredentials),
			err,
		)
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), conn, nil
}
