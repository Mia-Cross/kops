package scaleway

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	kopsv "k8s.io/kops"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	dns "k8s.io/kops/dnsprovider/pkg/dnsprovider/providers/scaleway"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/cloudinstances"
	"k8s.io/kops/upup/pkg/fi"

	account "github.com/scaleway/scaleway-sdk-go/api/account/v2alpha1"
	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const (
	TagNameEtcdClusterPrefix = "k8s.io/etcd/"
	TagNameRolePrefix        = "k8s.io/role/"
	TagClusterName           = "KubernetesCluster"
	TagRoleMaster            = "master"
	TagInstanceGroup         = "instance-group"
	TagRoleVolume            = "volume"
	TagRoleLoadBalancer      = "load-balancer"
)

// ScwCloud exposes all the interfaces required to operate on Scaleway resources
type ScwCloud interface {
	fi.Cloud

	Region() string
	Zone() string
	ProviderID() kops.CloudProviderID
	DNS() (dnsprovider.Interface, error)

	AccountService() *account.API
	DomainService() *domain.API
	InstanceService() *instance.API
	LBService() *lb.API
	VPCService() *vpc.API
	GatewayService() *vpcgw.API

	GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error)
	FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error)
	GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error)
	DeleteGroup(group *cloudinstances.CloudInstanceGroup) error
	FindVPCInfo(id string) (*fi.VPCInfo, error)
	DetachInstance(instance *cloudinstances.CloudInstance) error
	DeregisterInstance(instance *cloudinstances.CloudInstance) error
	DeleteInstance(i *cloudinstances.CloudInstance) error

	GetClusterGatewayNetworks(clusterName string) ([]*vpcgw.GatewayNetwork, error)
	GetClusterGateways(clusterName string) ([]*vpcgw.Gateway, error)
	GetClusterLoadBalancers(clusterName string) ([]*lb.LB, error)
	GetClusterServers(clusterName string) ([]*instance.Server, error)
	GetClusterVolumes(clusterName string) ([]*instance.Volume, error)
	GetClusterVPCs(clusterName string) ([]*vpc.PrivateNetwork, error)

	DeleteRecord(record *domain.Record, domainName string) error
	DeleteGateway(gateway *vpcgw.Gateway) error
	DeleteLoadBalancer(loadBalancer *lb.LB) error
	DeleteServer(server *instance.Server) error
	DeleteVolume(volume *instance.Volume) error
	DeleteVPC(vpc *vpc.PrivateNetwork) error
}

// static compile time check to validate ScwCloud's fi.Cloud Interface.
var _ fi.Cloud = &scwCloudImplementation{}

// scwCloudImplementation holds the scw.Client object to interact with Scaleway resources.
type scwCloudImplementation struct {
	client *scw.Client
	dns    dnsprovider.Interface
	//domainName string
	region scw.Region
	zone   scw.Zone
	tags   map[string]string

	accountAPI  *account.API
	domainAPI   *domain.API
	instanceAPI *instance.API
	lbAPI       *lb.API
	vpcAPI      *vpc.API
	gatewayAPI  *vpcgw.API
}

// NewScwCloud returns a Cloud, using the env vars SCW_ACCESS_KEY and SCW_SECRET_KEY
func NewScwCloud(region, zone string, tags map[string]string) (ScwCloud, error) {
	scwClient, err := scw.NewClient(
		scw.WithUserAgent("kubernetes-kops/"+kopsv.Version),
		scw.WithEnv(),
	)
	if err != nil {
		return nil, err
		// TODO: check if error is explicit enough when credentials are missing
	}

	return &scwCloudImplementation{
		client: scwClient,
		dns:    dns.NewProvider(scwClient),
		//domainName:  domainName,
		region:      scw.Region(region),
		zone:        scw.Zone(zone),
		tags:        tags,
		accountAPI:  account.NewAPI(scwClient),
		domainAPI:   domain.NewAPI(scwClient),
		instanceAPI: instance.NewAPI(scwClient),
		lbAPI:       lb.NewAPI(scwClient),
		vpcAPI:      vpc.NewAPI(scwClient),
		gatewayAPI:  vpcgw.NewAPI(scwClient),
	}, nil
}

func (s *scwCloudImplementation) Region() string {
	return string(s.region)
}

func (s *scwCloudImplementation) Zone() string {
	return string(s.zone)
}

func (s *scwCloudImplementation) ProviderID() kops.CloudProviderID {
	return kops.CloudProviderScaleway
}

func (s *scwCloudImplementation) DNS() (dnsprovider.Interface, error) {
	provider, err := dnsprovider.GetDnsProvider(dns.ProviderName, nil)
	if err != nil {
		return nil, fmt.Errorf("error building DNS provider: %w", err)
	}
	return provider, nil
}

func (s *scwCloudImplementation) AccountService() *account.API {
	return s.accountAPI
}

func (s *scwCloudImplementation) DomainService() *domain.API {
	return s.domainAPI
}

func (s *scwCloudImplementation) GatewayService() *vpcgw.API {
	return s.gatewayAPI
}

func (s *scwCloudImplementation) InstanceService() *instance.API {
	return s.instanceAPI
}

func (s *scwCloudImplementation) LBService() *lb.API {
	return s.lbAPI
}

func (s *scwCloudImplementation) VPCService() *vpc.API {
	return s.vpcAPI
}

// FindVPCInfo is not implemented yet, it's only here to satisfy the fi.Cloud interface
func (s *scwCloudImplementation) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	klog.V(8).Info("scaleway cloud provider FindVPCInfo not implemented yet")
	return nil, fmt.Errorf("scaleway cloud provider does not support vpc at this time")
}

func (s *scwCloudImplementation) DeleteInstance(i *cloudinstances.CloudInstance) error {
	// reach stopped state
	err := reachState(s.instanceAPI, s.zone, i.ID, instance.ServerStateStopped)
	if is404Error(err) {
		klog.V(8).Info("delete instance %s: instance was already deleted", i.ID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete instance %s: error reaching stopped state: %w", i.ID, err)
	}

	_, err = WaitForInstanceServer(s.instanceAPI, s.zone, i.ID)
	if err != nil {
		return fmt.Errorf("delete instance %s: error waiting for instance: %w", i.ID, err)
	}

	err = s.instanceAPI.DeleteServer(&instance.DeleteServerRequest{
		Zone:     s.zone,
		ServerID: i.ID,
	})
	if err != nil && !is404Error(err) {
		return fmt.Errorf("error deleting instance %s: %w", i.ID, err)
	}

	_, err = WaitForInstanceServer(s.instanceAPI, s.zone, i.ID)
	if err != nil && !is404Error(err) {
		return fmt.Errorf("delete instance %s: error waiting for instance: %w", i.ID, err)
	}

	return nil
}

func (s *scwCloudImplementation) DeregisterInstance(i *cloudinstances.CloudInstance) error {
	//TODO(Mia-Cross) implement me
	panic("implement me")
}

func (s *scwCloudImplementation) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
	toDelete := append(group.NeedUpdate, group.Ready...)
	for _, cloudInstance := range toDelete {
		err := s.DeleteInstance(cloudInstance)
		if err != nil {
			return fmt.Errorf("error deleting server %s: %w", cloudInstance.ID, err)
		}
	}
	return nil
}

func (s *scwCloudImplementation) DetachInstance(i *cloudinstances.CloudInstance) error {
	//TODO(Mia-Cross) implement me
	panic("implement me")
}

func (s *scwCloudImplementation) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	groups := make(map[string]*cloudinstances.CloudInstanceGroup)

	nodeMap := cloudinstances.GetNodeMap(nodes, cluster)

	serverGroups, err := findServerGroups(s, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find server groups: %w", err)
	}

	for igName, serverGroup := range serverGroups {
		var instanceGroup *kops.InstanceGroup
		for _, ig := range instancegroups {
			if igName == ig.Name {
				instanceGroup = ig
				break
			}
		}
		if instanceGroup == nil {
			if warnUnmatched {
				klog.Warningf("Server group %q has no corresponding instance group", igName)
			}
			continue
		}

		groups[instanceGroup.Name], err = buildCloudGroup(instanceGroup, serverGroup, nodeMap)
		if err != nil {
			return nil, fmt.Errorf("failed to build cloud group for instance group %q: %w", instanceGroup.Name, err)
		}
	}

	return groups, nil
}

func findServerGroups(s *scwCloudImplementation, clusterName string) (map[string][]*instance.Server, error) {
	servers, err := s.GetClusterServers(clusterName)
	if err != nil {
		return nil, err
	}

	serverGroups := make(map[string][]*instance.Server)
	for _, server := range servers {
		igName := ""
		for _, tag := range server.Tags {
			if strings.HasPrefix(tag, TagInstanceGroup) {
				igName = strings.TrimPrefix(tag, TagInstanceGroup+"=")
				break
			}
		}
		serverGroups[igName] = append(serverGroups[igName], server)
	}

	return serverGroups, nil
}

func buildCloudGroup(ig *kops.InstanceGroup, sg []*instance.Server, nodeMap map[string]*v1.Node) (*cloudinstances.CloudInstanceGroup, error) {
	cloudInstanceGroup := &cloudinstances.CloudInstanceGroup{
		HumanName:     ig.Name,
		InstanceGroup: ig,
		Raw:           sg,
		MinSize:       int(fi.Int32Value(ig.Spec.MinSize)),
		TargetSize:    int(fi.Int32Value(ig.Spec.MinSize)),
		MaxSize:       int(fi.Int32Value(ig.Spec.MaxSize)),
	}

	for _, server := range sg {
		status := cloudinstances.CloudInstanceStatusUpToDate
		cloudInstance, err := cloudInstanceGroup.NewCloudInstance(server.ID, status, nodeMap[server.ID])
		if err != nil {
			return nil, fmt.Errorf("failed to create cloud instance for server %s(%s): %w", server.Name, server.ID, err)
		}
		cloudInstance.State = cloudinstances.State(server.State)
		for _, tag := range server.Tags {
			if strings.HasPrefix(tag, TagNameRolePrefix) {
				cloudInstance.Roles = append(cloudInstance.Roles, strings.TrimPrefix(tag, TagNameRolePrefix))
			}
		}
		//TODO(Mia-Cross): add commercial type as cloudInstance.MachineType ??
		if server.PrivateIP != nil {
			cloudInstance.PrivateIP = *server.PrivateIP
		}
	}

	return cloudInstanceGroup, nil
}

func (s *scwCloudImplementation) FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error) {
	return nil, nil
}

func (s *scwCloudImplementation) GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
	var ingresses []fi.ApiIngressStatus
	name := "api." + cluster.Name

	responseLoadBalancers, err := s.lbAPI.ListLBs(&lb.ListLBsRequest{
		Region: scw.Region(s.Region()),
		Name:   &name,
	})
	if err != nil {
		return nil, fmt.Errorf("error finding load-balancers: %w", err)
	}
	if len(responseLoadBalancers.LBs) == 0 {
		// QUESTION: Is it serious ? I should probably log it
		klog.V(8).Infof("could not find any load-balancers for cluster %s", cluster.Name)
		return nil, nil
	}
	if len(responseLoadBalancers.LBs) > 1 {
		klog.V(4).Infof("more than 1 load-balancer with the name %s was found", name)
	}

	address := responseLoadBalancers.LBs[0].IP[0].IPAddress
	ingresses = append(ingresses, fi.ApiIngressStatus{IP: address})

	return ingresses, nil
}

func (s *scwCloudImplementation) GetClusterGatewayNetworks(privateNetworkID string) ([]*vpcgw.GatewayNetwork, error) {
	gwNetworks, err := s.gatewayAPI.ListGatewayNetworks(&vpcgw.ListGatewayNetworksRequest{
		Zone:             s.zone,
		PrivateNetworkID: scw.StringPtr(privateNetworkID),
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list gateway networks: %w", err)
	}
	return gwNetworks.GatewayNetworks, nil
}

func (s *scwCloudImplementation) GetClusterGateways(clusterName string) ([]*vpcgw.Gateway, error) {
	gws, err := s.gatewayAPI.ListGateways(&vpcgw.ListGatewaysRequest{
		Zone: s.zone,
		Tags: []string{TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list gateway networks: %w", err)
	}
	return gws.Gateways, nil
}

func (s *scwCloudImplementation) GetClusterLoadBalancers(clusterName string) ([]*lb.LB, error) {
	loadBalancerName := "api." + clusterName
	lbs, err := s.lbAPI.ListLBs(&lb.ListLBsRequest{
		Region: s.region,
		Name:   &loadBalancerName,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list load-balancers: %w", err)
	}
	return lbs.LBs, nil
}

func (s *scwCloudImplementation) GetClusterServers(clusterName string) ([]*instance.Server, error) {
	servers, err := s.instanceAPI.ListServers(&instance.ListServersRequest{
		Zone: s.zone,
		Tags: []string{TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	return servers.Servers, nil
}

func (s *scwCloudImplementation) GetClusterVolumes(clusterName string) ([]*instance.Volume, error) {
	volumes, err := s.instanceAPI.ListVolumes(&instance.ListVolumesRequest{
		Zone: s.zone,
		Tags: []string{TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}
	return volumes.Volumes, nil
}

func (s *scwCloudImplementation) GetClusterVPCs(clusterName string) ([]*vpc.PrivateNetwork, error) {
	vpcs, err := s.vpcAPI.ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
		Zone: s.zone,
		Tags: []string{TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list VPCs: %w", err)
	}
	return vpcs.PrivateNetworks, nil
}

func (s *scwCloudImplementation) DeleteRecord(record *domain.Record, domainName string) error {
	recordDeleteRequest := &domain.UpdateDNSZoneRecordsRequest{
		DNSZone: domainName,
		Changes: []*domain.RecordChange{
			{
				Delete: &domain.RecordChangeDelete{
					ID: scw.StringPtr(record.ID),
				},
			},
		},
	}
	_, err := s.domainAPI.UpdateDNSZoneRecords(recordDeleteRequest)
	if err != nil {
		return fmt.Errorf("failed to delete record %s: %w", record.Name, err)
	}
	return nil
}

func (s *scwCloudImplementation) DeleteGateway(gateway *vpcgw.Gateway) error {
	// We look for gateway connexions to private networks and detach them before deleting the gateway
	connexions, err := s.GetClusterGatewayNetworks(gateway.ID)
	if err != nil {
		return err
	}
	for _, connexion := range connexions {
		err := s.gatewayAPI.DeleteGatewayNetwork(&vpcgw.DeleteGatewayNetworkRequest{
			Zone:             s.zone,
			GatewayNetworkID: connexion.ID,
			CleanupDHCP:      true,
		})
		if err != nil {
			return fmt.Errorf("failed to detach gateway %s from private network: %w", gateway.ID, err)
		}
	}

	// We detach the IP of the gateway
	_, err = s.gatewayAPI.WaitForGateway(&vpcgw.WaitForGatewayRequest{
		GatewayID: gateway.ID,
		Zone:      s.zone,
	})
	if err != nil {
		return fmt.Errorf("error waiting for gateway: %w", err)
	}

	_, err = s.gatewayAPI.UpdateIP(&vpcgw.UpdateIPRequest{
		Zone:      s.zone,
		IPID:      gateway.IP.ID,
		GatewayID: scw.StringPtr(""),
	})
	if err != nil {
		return fmt.Errorf("failed to detach gateway IP: %w", err)
	}

	// We delete the IP of the gateway
	_, err = s.gatewayAPI.WaitForGateway(&vpcgw.WaitForGatewayRequest{
		GatewayID: gateway.ID,
		Zone:      s.zone,
	})
	if err != nil {
		return fmt.Errorf("error waiting for gateway: %w", err)
	}

	err = s.gatewayAPI.DeleteIP(&vpcgw.DeleteIPRequest{
		Zone: s.zone,
		IPID: gateway.IP.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete gateway IP: %w", err)
	}

	// We delete the gateway once it's in a stable state
	_, err = s.gatewayAPI.WaitForGateway(&vpcgw.WaitForGatewayRequest{
		GatewayID: gateway.ID,
		Zone:      s.zone,
	})
	if err != nil {
		return fmt.Errorf("error waiting for gateway: %w", err)
	}
	err = s.gatewayAPI.DeleteGateway(&vpcgw.DeleteGatewayRequest{
		Zone:        s.zone,
		GatewayID:   gateway.ID,
		CleanupDHCP: true,
	})
	if err != nil {
		return fmt.Errorf("failed to delete gateway %s: %w", gateway.ID, err)
	}

	return nil
}

func (s *scwCloudImplementation) DeleteLoadBalancer(loadBalancer *lb.LB) error {
	ipsToRelease := loadBalancer.IP

	// We delete the load-balancer once it's in a stable state
	_, err := s.lbAPI.WaitForLb(&lb.WaitForLBRequest{
		LBID:   loadBalancer.ID,
		Region: s.region,
	})
	if err != nil {
		return fmt.Errorf("error waiting for load-balancer: %w", err)
	}
	err = s.lbAPI.DeleteLB(&lb.DeleteLBRequest{
		Region: s.region,
		LBID:   loadBalancer.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete load-balancer %s: %w", loadBalancer.ID, err)
	}

	// We detach the IPs of the load-balancer
	for _, ip := range ipsToRelease {
		err := s.lbAPI.ReleaseIP(&lb.ReleaseIPRequest{
			Region: s.region,
			IPID:   ip.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to delete load-balancer IP: %w", err)
		}
	}
	return nil
}

func (s *scwCloudImplementation) DeleteServer(server *instance.Server) error {
	srv, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
		Zone:     s.zone,
		ServerID: server.ID,
	})
	if err != nil {
		klog.V(4).Infof("instance %s was already deleted", server.Name)
		return nil
	}

	// We detach the private network
	if len(srv.Server.PrivateNics) > 0 {
		err = s.instanceAPI.DeletePrivateNIC(&instance.DeletePrivateNICRequest{
			Zone:         s.zone,
			ServerID:     server.ID,
			PrivateNicID: srv.Server.PrivateNics[0].ID,
		})
		if err != nil {
			return fmt.Errorf("delete instance %s: error detaching private network: %w", server.ID, err)
		}
	}

	// If instance is running, we turn it off
	if srv.Server.State == "running" {
		_, err := s.instanceAPI.ServerAction(&instance.ServerActionRequest{
			Zone:     s.zone,
			ServerID: server.ID,
			Action:   "poweroff",
		})
		if err != nil {
			return fmt.Errorf("delete instance %s: error powering off instance: %w", server.ID, err)
		}
	}

	_, err = WaitForInstanceServer(s.instanceAPI, s.zone, server.ID)
	if err != nil {
		return fmt.Errorf("delete instance %s: error waiting for instance after power-off: %w", server.ID, err)
	}

	err = s.instanceAPI.DeleteServer(&instance.DeleteServerRequest{
		ServerID: server.ID,
		Zone:     s.zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete server %s: %w", server.ID, err)
	}

	return nil
}

func (s *scwCloudImplementation) DeleteVolume(volume *instance.Volume) error {
	err := s.instanceAPI.DeleteVolume(&instance.DeleteVolumeRequest{
		VolumeID: volume.ID,
		Zone:     s.zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete volume %s: %w", volume.ID, err)
	}
	return nil
}

func (s *scwCloudImplementation) DeleteVPC(privateNetwork *vpc.PrivateNetwork) error {
	err := s.vpcAPI.DeletePrivateNetwork(&vpc.DeletePrivateNetworkRequest{
		PrivateNetworkID: privateNetwork.ID,
		Zone:             s.zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC %s: %w", privateNetwork.ID, err)
	}
	return nil
}
