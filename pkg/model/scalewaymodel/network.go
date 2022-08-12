package scalewaymodel

import (
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scalewaytasks"
)

// NetworkModelBuilder configures network objects
type NetworkModelBuilder struct {
	*ScwModelContext
	Lifecycle fi.Lifecycle
}

func (b *NetworkModelBuilder) Build(c *fi.ModelBuilderContext) error {

	ipRange := b.Cluster.Spec.NetworkCIDR
	//if ipRange == "" {
	// no cidr specified, use the default vpc in DO that's always available
	// TODO(Mia-Cross): what about scaleway ?
	//return nil
	//}

	vpcName := "vpc-" + b.ClusterName()

	// Create a separate vpc for this cluster.
	vpc := &scalewaytasks.VPC{
		Name:      fi.String(vpcName),
		Zone:      fi.String(b.Cluster.Spec.Subnets[0].Zone),
		Lifecycle: b.Lifecycle,
		IPRange:   fi.String(ipRange),
	}
	c.AddTask(vpc)

	return nil
}
