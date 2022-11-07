package scalewaytasks

import (
	"fmt"
	"strings"

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

	VPCId *string // set if Cluster.Spec.NetworkID is
	//VPCName     *string // set if Cluster.Spec.NetworkCIDR is
	//NetworkCIDR *string // set if Cluster.Spec.NetworkCIDR is
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

	if l.LoadBalancerId == nil {
		return nil, nil
	}

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

	lbIP := loadBalancer.IP[0].IPAddress
	if len(loadBalancer.IP) > 1 {
		klog.V(4).Infof("multiple IPs found for load-balancer, using %s", lbIP)
	}

	return &LoadBalancer{
		Name:                &loadBalancer.Name,
		LoadBalancerId:      &loadBalancer.ID,
		LoadBalancerAddress: &lbIP,
		Tags:                loadBalancer.Tags,
		//AddressType:         loadBalancer.,
		//VSwitchId:           loadBalancer.,
		Lifecycle:    l.Lifecycle,
		ForAPIServer: l.ForAPIServer,
	}, nil
}

func (l *LoadBalancer) RenderScw(t *scaleway.ScwAPITarget, a, e, changes *LoadBalancer) error {
	lbService := t.Cloud.LBService()
	region := scw.Region(t.Cloud.Region())
	cloud := t.Cloud.(scaleway.ScwCloud)

	// Temporary fix to prevent the creation of a lb at update in DNS clusters
	// We should investigate why method shouldCreate at upup/pkg/fi/default_methods.go:76 returns true
	if e != nil && !strings.HasSuffix(cloud.ClusterName(e.Tags), ".k8s.local") {
		return nil
	}

	// We check if the load-balancer already exists
	lbs, err := lbService.ListLBs(&lb.ListLBsRequest{
		Region: region,
		Name:   e.Name,
	}, scw.WithAllPages())
	if err != nil {
		return fmt.Errorf("error listing existing load-balancers: %w", err)
	}

	if lbs.TotalCount > 0 {
		loadBalancer := lbs.LBs[0]
		lbIP := loadBalancer.IP[0].IPAddress
		if len(loadBalancer.IP) > 1 {
			klog.V(4).Infof("multiple IPs found for load-balancer, using %s", lbIP)
		}
		e.LoadBalancerId = &loadBalancer.ID
		e.LoadBalancerAddress = &lbIP
		return nil
	}

	loadBalancer, err := lbService.CreateLB(&lb.CreateLBRequest{
		Region: region,
		Name:   fi.StringValue(e.Name),
		IPID:   nil,
		Tags:   e.Tags,
		//Type:  e.Type,
	})
	if err != nil {
		return err
	}

	e.LoadBalancerId = &loadBalancer.ID

	// associate vpc to the loadbalancer if set
	//vpcs, err := cloud.GetClusterLoadBalancers(cloud.ClusterName(e.Tags))
	//if err != nil {
	//	return err
	//}
	//if len(vpcs) != 1 {
	//	return fmt.Errorf("could not find any VPC to link to the load-balancer")
	//}
	//vpc := vpcs[0]

	// How Do does it
	//if fi.StringValue(e.NetworkCIDR) != "" {
	//	vpcUUID, err := t.Cloud.GetVPCUUID(fi.StringValue(e.NetworkCIDR), fi.StringValue(e.VPCName))
	//	if err != nil {
	//		return fmt.Errorf("Error fetching vpcUUID from network cidr=%s", fi.StringValue(e.NetworkCIDR))
	//	}
	//} else if fi.StringValue(e.VPCId) != "" {
	//	vpcUUID = fi.StringValue(e.VPCId)
	//}

	if len(loadBalancer.IP) > 1 {
		klog.V(8).Infof("got more more than 1 IP for LB (got %d)", len(loadBalancer.IP))
	}
	ip := (*loadBalancer.IP[0]).IPAddress
	e.LoadBalancerAddress = &ip

	//backEnd, err := lbService.CreateBackend(&lb.CreateBackendRequest{
	//	Region:               region,
	//	LBID:                 loadBalancer.ID,
	//	Name:                 "lb-backend",
	//	ForwardProtocol:      "tcp",
	//	ForwardPort:          443,
	//	ForwardPortAlgorithm: roundrobin,
	//	StickySessions:       "none",
	//	//StickySessionsCookieName: "",
	//	HealthCheck:        nil,
	//	ServerIP:           nil,
	//	TimeoutServer:      nil,
	//	TimeoutConnect:     nil,
	//	TimeoutTunnel:      nil,
	//	OnMarkedDownAction: "",
	//	ProxyProtocol:      "",
	//	FailoverHost:       nil,
	//	SslBridging:        nil,
	//})

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
