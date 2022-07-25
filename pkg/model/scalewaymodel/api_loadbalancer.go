package scalewaymodel

import (
	"k8s.io/kops/upup/pkg/fi"
)

// APILoadBalancerModelBuilder builds a LoadBalancer for accessing the API
type APILoadBalancerModelBuilder struct {
	*ScwModelContext
	Lifecycle fi.Lifecycle
}

var _ fi.ModelBuilder = &APILoadBalancerModelBuilder{}

func (b *APILoadBalancerModelBuilder) Build(c *fi.ModelBuilderContext) error {
	//// Configuration where a load balancer fronts the API
	//if !b.UseLoadBalancerForAPI() {
	//	return nil
	//}

	//lbSpec := b.Cluster.Spec.API.LoadBalancer
	//if lbSpec == nil {
	//	// Skipping API LB creation; not requested in Spec
	//	return nil
	//}

	//switch lbSpec.Type {
	//case kops.LoadBalancerTypePublic:
	//// OK

	//case kops.LoadBalancerTypeInternal:
	//	return fmt.Errorf("internal LoadBalancers are not yet supported by kops on Scaleway")

	//default:
	//	return fmt.Errorf("unhandled LoadBalancer type %q", lbSpec.Type)
	//}

	////clusterName := strings.Replace(b.ClusterName(), ".", "-", -1)
	////loadbalancerName := "api-" + clusterName
	////clusterMasterTag := do.TagKubernetesClusterMasterPrefix + ":" + clusterName

	//// Create LoadBalancer for API LB
	////loadbalancer := &dotasks.LoadBalancer{ //TODO(jtherin): implement loadbalancer
	////	Name:       fi.String(loadbalancerName),
	////	Region:     fi.String(b.Cluster.Spec.Subnets[0].Region),
	////	DropletTag: fi.String(clusterMasterTag),
	////	Lifecycle:  b.Lifecycle,
	////}
	////c.AddTask(loadbalancer)

	//// Temporarily do not know the role of the following function
	//if dns.IsGossipHostname(b.Cluster.Name) || b.UsePrivateDNS() {
	//	// Ensure the LB hostname is included in the TLS certificate,
	//	// if we're not going to use an alias for it
	//	//loadbalancer.ForAPIServer = true
	//}

	return nil

}
