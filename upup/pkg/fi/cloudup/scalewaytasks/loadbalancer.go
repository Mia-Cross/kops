package scalewaytasks

import (
	"fmt"
	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"

	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// LoadBalancer represents a Scaleway LoadBalancer
// +kops:fitask
type LoadBalancer struct {
	Name                *string
	LoadBalancerId      *string
	AddressType         *string
	VSwitchId           *string
	LoadBalancerAddress *string
	Lifecycle           fi.Lifecycle
	Tags                []string
	ForAPIServer        bool
}

var _ fi.CompareWithID = &LoadBalancer{}
var _ fi.HasAddress = &LoadBalancer{}

func (l *LoadBalancer) CompareWithID() *string {
	return l.LoadBalancerId
}

func (l *LoadBalancer) IsForAPIServer() bool {
	return l.ForAPIServer
}

func (l *LoadBalancer) Run(context *fi.Context) error {
	return fi.DefaultDeltaRunMethod(l, context)
}

func (l *LoadBalancer) FindAddresses(context *fi.Context) ([]string, error) {
	cloud := context.Cloud.(scaleway.ScwCloud)
	lbService := cloud.LBService()

	loadBalancer, err := lbService.GetLB(&lb.GetLBRequest{
		Region: scw.Region(cloud.Region()),
		LBID:   fi.StringValue(l.LoadBalancerId),
	})
	if err != nil {
		return nil, err
	}

	addresses := []string(nil)
	for _, address := range loadBalancer.IP {
		addresses = append(addresses, address.IPAddress)
	}

	return addresses, nil
}

func (l *LoadBalancer) Find(context *fi.Context) (*LoadBalancer, error) {
	if fi.StringValue(l.LoadBalancerId) == "" {
		// Loadbalancer = nil if not found
		return nil, nil
	}
	cloud := context.Cloud.(scaleway.ScwCloud)
	lbService := cloud.LBService()

	loadBalancer, err := lbService.GetLB(&lb.GetLBRequest{
		Region: scw.Region(cloud.Region()),
		LBID:   fi.StringValue(l.LoadBalancerId),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting load-balancer %s: %s", fi.StringValue(l.LoadBalancerId), err)
	}

	return &LoadBalancer{
		Name:           &loadBalancer.Name,
		LoadBalancerId: &loadBalancer.ID,
		//AddressType:         loadBalancer.,
		//VSwitchId:           loadBalancer.,
		//LoadBalancerAddress: loadBalancer.,
		Lifecycle: l.Lifecycle,
		Tags:      loadBalancer.Tags,
		//ForAPIServer:        loadBalancer.,
	}, nil
}

func (l *LoadBalancer) RenderScw(t *scaleway.ScwAPITarget, a, e, changes *LoadBalancer) error {
	lbService := t.Cloud.LBService()

	//if a == nil {
	loadBalancer, err := lbService.CreateLB(&lb.CreateLBRequest{
		Region: scw.Region(t.Cloud.Region()),
		Name:   fi.StringValue(e.Name),
		IPID:   nil,
		Tags:   e.Tags,
		//Type:                  e.Type,
	})
	if err != nil {
		return err
	}
	//}

	e.LoadBalancerId = &loadBalancer.ID

	if len(loadBalancer.IP) > 1 {
		klog.V(8).Infof("got more more than 1 IP for LB (got %d)", len(loadBalancer.IP))
	}
	ip := (*loadBalancer.IP[0]).IPAddress
	e.LoadBalancerAddress = &ip

	// TODO: handle changes

	return nil
}

func (_ *LoadBalancer) CheckChanges(a, e, changes *LoadBalancer) error {
	if a != nil {
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
		if changes.LoadBalancerId != nil {
			return fi.CannotChangeField("ID")
		}
	} else {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
	}
	return nil
}
