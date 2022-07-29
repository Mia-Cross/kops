package scalewaytasks

import (
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
	Tags                map[string]string
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

	lb, err := lbService.GetLB(&lb.GetLBRequest{
		Region: scw.Region(cloud.Region()),
		LBID:   fi.StringValue(l.LoadBalancerId),
	})
	if err != nil {
		return nil, err
	}

	addresses := []string(nil)
	for _, address := range lb.IP {
		addresses = append(addresses, address.IPAddress)
	}

	return addresses, nil
}

//func (l *LoadBalancer) FindIPAddress(context *fi.Context) (*string, error) {
//	panic("implement me")
//}

func (l *LoadBalancer) RenderScw(t *scaleway.ScwAPITarget, a, e, changes *LoadBalancer) error {
	lbService := t.Cloud.LBService()

	//if a == nil {
	lb, err := lbService.CreateLB(&lb.CreateLBRequest{
		Region: scw.Region(t.Cloud.Region()),
		Name:   fi.StringValue(e.Name),
		IPID:   nil,
		//Tags:                  e.Tags,
		//Type:                  e.Type,
	})
	if err != nil {
		return err
	}
	//}

	e.LoadBalancerId = &lb.ID
	//e.LoadBalancerAddress = &lb.IP

	// TODO: handle changes

	return nil
}
