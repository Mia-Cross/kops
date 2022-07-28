package scalewaytasks

import (
	"fmt"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/pki"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"strings"

	account "github.com/scaleway/scaleway-sdk-go/api/account/v2alpha1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// +kops:fitask
type SSHKey struct {
	ID                 *string
	Name               *string
	Lifecycle          fi.Lifecycle
	PublicKey          *fi.Resource
	KeyPairFingerPrint *string
}

var _ fi.CompareWithID = &SSHKey{}

func (s *SSHKey) CompareWithID() *string {
	return s.Name
}

func (s *SSHKey) Find(c *fi.Context) (*SSHKey, error) {
	cloud := c.Cloud.(scaleway.ScwCloud)

	keysReq, err := cloud.AccountService().ListSSHKeys(&account.ListSSHKeysRequest{
		Name: s.Name,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("error listing SSHKeys: %v", err)
	}

	if len(keysReq.SSHKeys) == 0 {
		return nil, nil
	}

	if len(keysReq.SSHKeys) != 1 {
		return nil, fmt.Errorf("found multiple SSHKeys with Name %q", *s.Name)
	}

	klog.V(2).Infof("found matching SSHKey with name: %q", *s.Name)
	k := keysReq.SSHKeys[0]
	actual := &SSHKey{
		ID:                 fi.String(k.ID),
		Name:               fi.String(k.Name),
		KeyPairFingerPrint: fi.String(k.Fingerprint),
	}

	// Avoid spurious changes
	if strings.Contains(fi.StringValue(actual.KeyPairFingerPrint), fi.StringValue(s.KeyPairFingerPrint)) {
		klog.V(2).Infof("SSH key fingerprints match; assuming public keys match")
		actual.PublicKey = s.PublicKey
		actual.KeyPairFingerPrint = s.KeyPairFingerPrint
	} else {
		klog.V(2).Infof("Computed SSH key fingerprint mismatch: %q %q", fi.StringValue(s.KeyPairFingerPrint), fi.StringValue(actual.KeyPairFingerPrint))
	}

	// Ignore "system" fields
	actual.Lifecycle = s.Lifecycle

	return actual, nil
}

func (s *SSHKey) Run(c *fi.Context) error {
	if s.KeyPairFingerPrint == nil && s.PublicKey != nil {
		publicKey, err := fi.ResourceAsString(*s.PublicKey)
		if err != nil {
			return fmt.Errorf("error reading SSH public key: %v", err)
		}

		keyPairFingerPrint, err := pki.ComputeOpenSSHKeyFingerprint(publicKey)
		if err != nil {
			return fmt.Errorf("error computing key fingerprint for SSH key: %v", err)
		}
		klog.V(2).Infof("Computed SSH key fingerprint as %q", keyPairFingerPrint)
		s.KeyPairFingerPrint = &keyPairFingerPrint
	}
	return fi.DefaultDeltaRunMethod(s, c)
}

func (s *SSHKey) CheckChanges(a, e, changes *SSHKey) error {
	if a != nil {
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
	}
	return nil
}

func (*SSHKey) RenderScw(c *fi.Context, a, e, changes *SSHKey) error {
	cloud := c.Cloud.(scaleway.ScwCloud)
	if a == nil {
		name := fi.StringValue(e.Name)
		if name == "" {
			return fi.RequiredField("Name")
		}
		klog.V(2).Infof("Creating Keypair with name: %q", fi.StringValue(e.Name))
		keyArgs := &account.CreateSSHKeyRequest{
			Name: fi.StringValue(e.Name),
		}
		if e.PublicKey != nil {
			d, err := fi.ResourceAsString(*e.PublicKey)
			if err != nil {
				return fmt.Errorf("error rendering SSHKey PublicKey: %v", err)
			}
			keyArgs.PublicKey = d
		}
		v, err := cloud.AccountService().CreateSSHKey(keyArgs)
		if err != nil {
			return fmt.Errorf("error creating keypair: %v", err)
		}
		e.KeyPairFingerPrint = fi.String(v.Fingerprint)
		klog.V(2).Infof("Creating a new Scaleway keypair, id=%q fingerprint=%q", v.ID, v.Fingerprint)
		return nil
	}
	e.KeyPairFingerPrint = a.KeyPairFingerPrint
	klog.V(2).Infof("Using an existing Scaleway keypair, fingerprint=%q", fi.StringValue(e.KeyPairFingerPrint))
	return nil
}
