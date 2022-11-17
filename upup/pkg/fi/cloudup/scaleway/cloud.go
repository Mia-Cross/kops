package scaleway

import (
	"fmt"
	"os"
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
	TagClusterName           = "kops.k8s.io-cluster"
	KopsUserAgentPrefix      = "kubernetes-kops/"
	TagNameEtcdClusterPrefix = "k8s.io-etcd"
	TagNameRolePrefix        = "k8s.io-role"
	TagRoleMaster            = "master"
	TagInstanceGroup         = "instance-group"
	TagRoleVolume            = "volume"
	TagRoleLoadBalancer      = "load-balancer"
	TagNeedsUpdate           = "kops.k8s.io-needs-update"
)

// ScwCloud exposes all the interfaces required to operate on Scaleway resources
type ScwCloud interface {
	fi.Cloud

	Region() string
	Zone() string
	ProviderID() kops.CloudProviderID
	DNS() (dnsprovider.Interface, error)
	ClusterName(tags []string) string

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
	GetClusterServers(clusterName string, serverName *string) ([]*instance.Server, error)
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
func NewScwCloud(tags map[string]string) (ScwCloud, error) {
	displayEnv()

	region, err := scw.ParseRegion(os.Getenv("SCW_DEFAULT_REGION"))
	if err != nil {
		return nil, fmt.Errorf("error parsing SCW_DEFAULT_REGION: %w", err)
	}
	zone, err := scw.ParseZone(os.Getenv("SCW_DEFAULT_ZONE"))
	if err != nil {
		return nil, fmt.Errorf("error parsing SCW_DEFAULT_ZONE: %w", err)
	}

	// We make sure that the credentials env vars are defined
	scwAccessKey := os.Getenv("SCW_ACCESS_KEY")
	if scwAccessKey == "" {
		return nil, fmt.Errorf("SCW_ACCESS_KEY has to be set as an environment variable")
	}
	scwSecretKey := os.Getenv("SCW_SECRET_KEY")
	if scwSecretKey == "" {
		return nil, fmt.Errorf("SCW_SECRET_KEY has to be set as an environment variable")
	}
	scwProjectID := os.Getenv("SCW_DEFAULT_PROJECT_ID")
	if scwProjectID == "" {
		return nil, fmt.Errorf("SCW_DEFAULT_PROJECT_ID has to be set as an environment variable")
	}

	scwClient, err := scw.NewClient(
		scw.WithUserAgent("kubernetes-kops/"+kopsv.Version),
		scw.WithEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("error building client for Scaleway Cloud: %w", err)
	}

	return &scwCloudImplementation{
		client:      scwClient,
		dns:         dns.NewProvider(scwClient),
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

func (s *scwCloudImplementation) ClusterName(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, TagClusterName) {
			return strings.TrimPrefix(tag, TagClusterName+"=")
		}
	}
	return ""
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

// FindVPCInfo looks up the specified VPC by id, returning info if found, otherwise (nil, nil).
func (s *scwCloudImplementation) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	klog.V(8).Info("scaleway cloud provider FindVPCInfo not implemented yet")
	return nil, fmt.Errorf("scaleway cloud provider does not support vpc at this time")
}

func (s *scwCloudImplementation) DeleteInstance(i *cloudinstances.CloudInstance) error {
	server, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
		Zone:     s.zone,
		ServerID: i.ID,
	})
	if err != nil {
		if is404Error(err) {
			klog.V(4).Infof("error deleting cloud instance %s of group %s : instance was already deleted", i.ID, i.CloudInstanceGroup.HumanName)
			return nil
		}
		return fmt.Errorf("error deleting cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
	}

	err = s.DeleteServer(server.Server)
	if err != nil {
		return fmt.Errorf("error deleting cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
	}

	return nil
}

// DeregisterInstance drains a cloud instance and load-balancers.
func (s *scwCloudImplementation) DeregisterInstance(i *cloudinstances.CloudInstance) error {
	server, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
		Zone:     s.zone,
		ServerID: i.ID,
	})
	if err != nil {
		return fmt.Errorf("error deregistering cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
	}

	// We remove the instance's IP from load-balancers
	lbs, err := s.GetClusterLoadBalancers(s.ClusterName(server.Server.Tags))
	if err != nil {
		return fmt.Errorf("error deregistering cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
	}
	for _, loadBalancer := range lbs {
		backEnds, err := s.lbAPI.ListBackends(&lb.ListBackendsRequest{
			Region: s.region,
			LBID:   loadBalancer.ID,
		}, scw.WithAllPages())
		if err != nil {
			return fmt.Errorf("eerror deregistering cloud instance %s of group %s: error listing load-balancer's back-ends for instance creation: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
		}
		for _, backEnd := range backEnds.Backends {
			for _, serverIP := range backEnd.Pool {
				if serverIP == fi.StringValue(server.Server.PrivateIP) {
					_, err := s.lbAPI.RemoveBackendServers(&lb.RemoveBackendServersRequest{
						Region:    s.region,
						BackendID: backEnd.ID,
						ServerIP:  []string{serverIP},
					})
					if err != nil {
						return fmt.Errorf("error deregistering cloud instance %s of group %s: error removing IP from lb %w", i.ID, i.CloudInstanceGroup.HumanName, err)
					}
				}
			}
		}
	}

	return nil
}

// DeleteGroup deletes the cloud resources that make up a CloudInstanceGroup, including the instances.
func (s *scwCloudImplementation) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
	toDelete := append(group.NeedUpdate, group.Ready...)
	for _, cloudInstance := range toDelete {
		err := s.DeleteInstance(cloudInstance)
		if err != nil {
			return fmt.Errorf("error deleting group %q: %w", group.HumanName, err)
		}
	}
	return nil
}

// DetachInstance causes a cloud instance to no longer be counted against the group's size limits.
func (s *scwCloudImplementation) DetachInstance(i *cloudinstances.CloudInstance) error {
	cloudIG := i.CloudInstanceGroup
	newReadyInstances := []*cloudinstances.CloudInstance(nil)
	found := false

	for _, cloudInstance := range cloudIG.Ready {
		if cloudInstance.ID != i.ID {
			newReadyInstances = append(newReadyInstances, cloudInstance)
		} else {
			found = true
			cloudIG.NeedUpdate = append(cloudIG.NeedUpdate, cloudInstance)
			klog.V(4).Infof("Detached instance %s from group %q", i.ID, cloudIG.HumanName)
		}
	}

	if !found {
		return fmt.Errorf("could not detach instance %s from group %q: not found in CloudInstanceGroup.Ready", i.ID, cloudIG.HumanName)
	}
	return nil

}

// GetCloudGroups returns a map of cloud instances that back a kops cluster.
// Detached instances must be returned in the NeedUpdate slice.
func (s *scwCloudImplementation) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	groups := make(map[string]*cloudinstances.CloudInstanceGroup)

	nodeMap := cloudinstances.GetNodeMap(nodes, cluster)

	serverGroups, err := findServerGroups(s, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find server groups: %w", err)
	}

	for _, ig := range instancegroups {
		serverGroup, ok := serverGroups[ig.Name]
		if !ok {
			if warnUnmatched {
				klog.Warningf("Server group %q has no corresponding instance group", ig.Name)
			}
			continue
		}

		groups[ig.Name], err = buildCloudGroup(ig, serverGroup, nodeMap)
		if err != nil {
			return nil, fmt.Errorf("failed to build cloud group for instance group %q: %w", ig.Name, err)
		}
	}

	return groups, nil
}

func findServerGroups(s *scwCloudImplementation, clusterName string) (map[string][]*instance.Server, error) {
	//TODO(Mia-Cross): maybe refactor that because ig name could be given here instead of trimming the list after
	servers, err := s.GetClusterServers(clusterName, nil)
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
		for _, tag := range server.Tags {
			if tag == TagNeedsUpdate {
				status = cloudinstances.CloudInstanceStatusNeedsUpdate
			}
		}

		cloudInstance, err := cloudInstanceGroup.NewCloudInstance(server.ID, status, nodeMap[server.ID])
		if err != nil {
			return nil, fmt.Errorf("failed to create cloud instance for server %s(%s): %w", server.Name, server.ID, err)
		}
		cloudInstance.State = cloudinstances.State(server.State)
		for _, tag := range server.Tags {
			if strings.HasPrefix(tag, TagNameRolePrefix) {
				cloudInstance.Roles = append(cloudInstance.Roles, strings.TrimPrefix(tag, TagNameRolePrefix+"="))
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

	for _, loadBalancer := range responseLoadBalancers.LBs {
		for _, lbIP := range loadBalancer.IP {
			ingresses = append(ingresses, fi.ApiIngressStatus{IP: lbIP.IPAddress})
		}
	}

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

func (s *scwCloudImplementation) GetClusterServers(clusterName string, serverName *string) ([]*instance.Server, error) {
	request := &instance.ListServersRequest{
		Zone: s.zone,
		Name: serverName,
		Tags: []string{TagClusterName + "=" + clusterName},
	}
	servers, err := s.instanceAPI.ListServers(request, scw.WithAllPages())
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

	// We wait for the gateway to be deleted
	for {
		_, err := s.gatewayAPI.GetGateway(&vpcgw.GetGatewayRequest{
			Zone:      s.zone,
			GatewayID: gateway.ID,
		})
		if is404Error(err) {
			break
		}
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

	for {
		_, err := s.lbAPI.GetLB(&lb.GetLBRequest{
			Region: s.region,
			LBID:   loadBalancer.ID,
		})
		if is404Error(err) {
			break
		}
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
		if is404Error(err) {
			klog.V(4).Infof("instance %s was already deleted", server.Name)
			return nil
		}
		return err
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
	if srv.Server.State == instance.ServerStateRunning {
		_, err := s.instanceAPI.ServerAction(&instance.ServerActionRequest{
			Zone:     s.zone,
			ServerID: server.ID,
			Action:   instance.ServerActionPoweroff,
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

	for {
		_, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
			Zone:     s.zone,
			ServerID: server.ID,
		})
		if is404Error(err) {
			break
		}
	}

	for i := range server.Volumes {
		err = s.instanceAPI.DeleteVolume(&instance.DeleteVolumeRequest{
			Zone:     s.zone,
			VolumeID: server.Volumes[i].ID,
		})
		if err != nil {
			return fmt.Errorf("delete instance %s: error deleting volume %s: %w", server.ID, server.Volumes[i].ID, err)
		}
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
