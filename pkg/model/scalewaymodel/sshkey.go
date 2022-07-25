package scalewaymodel

import (
	"k8s.io/kops/upup/pkg/fi"
)

// SSHKeyModelBuilder configures SSH objects
type SSHKeyModelBuilder struct {
	*ScwModelContext
	Lifecycle fi.Lifecycle
}

var _ fi.ModelBuilder = &SSHKeyModelBuilder{}

func (b *SSHKeyModelBuilder) Build(c *fi.ModelBuilderContext) error {
	//name, err := b.SSHKeyName()
	//if err != nil {
	//	return err
	//}
	//
	//t := &scalewaytasks.SSHKey{
	//	Name:      fi.String(name),
	//	Lifecycle: b.Lifecycle,
	//	PublicKey: fi.WrapResource(fi.NewStringResource(string(b.SSHPublicKeys[0]))),
	//}
	//c.AddTask(t)

	return nil
}
