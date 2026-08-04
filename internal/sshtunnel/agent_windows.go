package sshtunnel

import (
	"fmt"
	"io"
	"os"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// agentPipe is where Windows' own OpenSSH agent listens. It is a named pipe rather than a socket,
// which is the whole reason this file is split from the one beside it — `SSH_AUTH_SOCK` means
// nothing here, and the agent is a service that is either running or is not.
const agentPipe = `\\.\pipe\openssh-ssh-agent`

// agentAuth offers the keys the reader has already unlocked. It is the one authentication of the
// three where this app never holds a key or a pass phrase: the agent does the signing, and the
// private key never leaves it.
//
// The pipe has to stay open for the whole handshake, since that is when the signing happens, so
// it is handed back to be closed by the caller rather than closed here.
func agentAuth() (ssh.AuthMethod, io.Closer, error) {
	pipe, err := os.OpenFile(agentPipe, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w: no SSH agent — start the OpenSSH Authentication Agent service: %w",
			domain.ErrTunnelCredentials, err,
		)
	}
	return ssh.PublicKeysCallback(agent.NewClient(pipe).Signers), pipe, nil
}
