package scalewaymodel

import (
	"fmt"

	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/dns"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"k8s.io/kops/upup/pkg/fi/cloudup/scalewaytasks"
)

// APILoadBalancerModelBuilder builds a LoadBalancer for accessing the API
type APILoadBalancerModelBuilder struct {
	*ScwModelContext
	Lifecycle fi.Lifecycle
}

var _ fi.ModelBuilder = &APILoadBalancerModelBuilder{}

func (b *APILoadBalancerModelBuilder) Build(c *fi.ModelBuilderContext) error {
	// Configuration where a load balancer fronts the API
	if !b.UseLoadBalancerForAPI() {
		return nil
	}

	lbSpec := b.Cluster.Spec.API.LoadBalancer
	if lbSpec == nil {
		// Skipping API LB creation; not requested in Spec
		return nil
	}

	switch lbSpec.Type {
	case kops.LoadBalancerTypePublic:
		klog.V(8).Infof("Using public load-balancer")
	case kops.LoadBalancerTypeInternal:
		return fmt.Errorf("internal LoadBalancers are not yet supported by kops on Scaleway")
	default:
		return fmt.Errorf("unhandled LoadBalancer type %q", lbSpec.Type)
	}

	loadBalancerName := "api." + b.ClusterName()

	// Create LoadBalancer for API LB
	loadBalancer := &scalewaytasks.LoadBalancer{ //TODO(jtherin): implement loadBalancer
		Name: fi.String(loadBalancerName),
		//Zone:     fi.String(b.Cluster.Spec.Subnets[0].Zone),
		Lifecycle: b.Lifecycle,
		Tags: []string{
			scaleway.TagClusterName + "=" + b.ClusterName(),
			scaleway.TagNameRolePrefix + "=" + scaleway.TagRoleLoadBalancer, // QUESTION : is this tag useful or not ?
		},
	}

	//if b.Cluster.Spec.NetworkID != "" {
	//	loadBalancer.VPCId = fi.String(b.Cluster.Spec.NetworkID)
	//} else if b.Cluster.Spec.NetworkCIDR != "" {
	//	loadBalancer.VPCName = fi.String(b.ClusterName())
	//	loadBalancer.NetworkCIDR = fi.String(b.Cluster.Spec.NetworkCIDR)
	//}

	c.AddTask(loadBalancer)

	// Temporarily do not know the role of the following function
	if dns.IsGossipClusterName(b.Cluster.Name) || b.UsePrivateDNS() {
		// Ensure the LB hostname is included in the TLS certificate,
		// if we're not going to use an alias for it
		loadBalancer.ForAPIServer = true
	}

	return nil

}
