package scalewaymodel

import (
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi/cloudup/scalewaytasks"
)

// Scaleway Model Context
type ScwModelContext struct {
	*model.KopsModelContext
}

func (b *ScwModelContext) LinkToNetwork() *scalewaytasks.Network {
	name := b.ClusterName()
	return &scalewaytasks.Network{Name: &name}
}
