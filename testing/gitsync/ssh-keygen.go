package gitsynctesting

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

func generateSshKeyPair(
	passphrase []byte,
	comment string,
) (publicKeyBytes []byte, privateKeyBytes []byte, err error) {
	var publicKey ed25519.PublicKey
	var privateKey ed25519.PrivateKey
	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		err = fmt.Errorf("failed to generate new ed25519 key pair: %w", err)
		return
	}

	{
		var pemBlock *pem.Block
		pemBlock, err = ssh.MarshalPrivateKeyWithPassphrase(privateKey, comment, passphrase)
		if err != nil {
			err = fmt.Errorf("failed to marshal private key: %w", err)
			return
		}

		privateKeyBytes = pem.EncodeToMemory(pemBlock)
	}

	{
		var sshPublicKey ssh.PublicKey
		sshPublicKey, err = ssh.NewPublicKey(publicKey)
		if err != nil {
			err = fmt.Errorf("failed to create new SSH public key: %w", err)
			return
		}
		publicKeyBytes = ssh.MarshalAuthorizedKey(sshPublicKey)
	}
	return
}
