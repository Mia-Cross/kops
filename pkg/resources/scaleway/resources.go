package scaleway

import (
	"fmt"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"

	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const (
	resourceTypeDNSRecord = "dns-record"
	resourceTypeGateway   = "gateway"
	//resourceTypeGatewayNetwork = "gateway-network"
	resourceTypeLoadBalancer = "load-balancer"
	resourceTypeVolume       = "volume"
	resourceTypeServer       = "server"
	resourceTypeVPC          = "vpc"
)

type listFn func(fi.Cloud, string) ([]*resources.Resource, error)

func ListResources(cloud scaleway.ScwCloud, clusterName string) (map[string]*resources.Resource, error) {
	resourceTrackers := make(map[string]*resources.Resource)

	listFunctions := []listFn{
		listDNSRecords,
		listGateways,
		//listGatewayNetworks,
		listLoadBalancers,
		listServers,
		listVolumes,
		listVPCs,
	}

	for _, fn := range listFunctions {
		rt, err := fn(cloud, clusterName)
		if err != nil {
			return nil, err
		}
		for _, t := range rt {
			resourceTrackers[t.Type+":"+t.ID] = t
		}
	}

	return resourceTrackers, nil
}

func listDNSRecords(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	names := strings.SplitN(clusterName, ".", 2)
	clusterNameShort := names[0]
	domainName := names[1]

	records, err := c.DomainService().ListDNSZoneRecords(&domain.ListDNSZoneRecordsRequest{
		DNSZone: domainName,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %s", err)
	}

	//if domainName == "" {
	//	if strings.HasSuffix(clusterName, ".k8s.local") {
	//		klog.Info("Domain Name is empty. Ok to have an empty domain name since cluster is configured as gossip cluster.")
	//		return nil, nil
	//	}
	//	return nil, fmt.Errorf("failed to find domain for cluster: %s", clusterName)
	//}

	resourceTrackers := []*resources.Resource(nil)
	for _, record := range records.Records {
		if !strings.HasSuffix(record.Name, clusterNameShort) {
			continue
		}
		resourceTracker := &resources.Resource{
			Name: record.Name,
			ID:   record.ID,
			Type: resourceTypeDNSRecord,
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteRecord(cloud, tracker, domainName)
			},
			Obj: record,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listGateways(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	gws, err := c.GetClusterGateways(clusterName)
	if err != nil {
		return nil, err
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, gw := range gws {
		resourceTracker := &resources.Resource{
			Name:   gw.Name,
			ID:     gw.ID,
			Type:   resourceTypeGateway,
			Blocks: []string{resourceTypeVPC},
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteGateway(cloud, tracker)
			},
			Obj: gw,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

//func listGatewayNetworks(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
//	c := cloud.(scaleway.ScwCloud)
//	gwNetworks, err := c.GetClusterGatewayNetworks(clusterName)
//	if err != nil {
//		return nil, err
//	}
//
//	resourceTrackers := []*resources.Resource(nil)
//	for _, gwNetwork := range gwNetworks {
//		resourceTracker := &resources.Resource{
//			Name:   "gw-network-" + gwNetwork.ID,
//			ID:     gwNetwork.ID,
//			Type:   resourceTypeGatewayNetwork,
//			Blocks: []string{resourceTypeGateway},
//			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
//				return deleteGatewayNetwork(cloud, tracker)
//			},
//			Obj: gwNetwork,
//		}
//		resourceTrackers = append(resourceTrackers, resourceTracker)
//	}
//
//	return resourceTrackers, nil
//}

func listLoadBalancers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	lbs, err := c.GetClusterLoadBalancers(clusterName)
	if err != nil {
		return nil, err
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, loadBalancer := range lbs {
		resourceTracker := &resources.Resource{
			Name: loadBalancer.Name,
			ID:   loadBalancer.ID,
			Type: resourceTypeLoadBalancer,
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteLoadBalancer(cloud, tracker)
			},
			Obj: loadBalancer,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listServers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	servers, err := c.GetClusterServers(clusterName)
	if err != nil {
		return nil, err
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, server := range servers {
		resourceTracker := &resources.Resource{
			Name:   server.Name,
			ID:     server.ID,
			Type:   resourceTypeServer,
			Blocks: []string{resourceTypeVolume, resourceTypeVPC},
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteServer(cloud, tracker)
			},
			Obj: server,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listVolumes(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	volumes, err := c.GetClusterVolumes(clusterName)
	if err != nil {
		return nil, err
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, volume := range volumes {
		resourceTracker := &resources.Resource{
			Name:    volume.Name,
			ID:      volume.ID,
			Type:    resourceTypeVolume,
			Blocked: []string{resourceTypeServer},
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteVolume(cloud, tracker)
			},
			Obj: volume,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listVPCs(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	vpcs, err := c.GetClusterVPCs(clusterName)
	if err != nil {
		return nil, err
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, vpc := range vpcs {
		resourceTracker := &resources.Resource{
			Name:    vpc.Name,
			ID:      vpc.ID,
			Type:    resourceTypeVPC,
			Blocked: []string{resourceTypeGateway, resourceTypeServer},
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteVPC(cloud, tracker)
			},
			Obj: vpc,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func deleteRecord(cloud fi.Cloud, tracker *resources.Resource, domainName string) error {
	c := cloud.(scaleway.ScwCloud)

	recordDeleteRequest := &domain.UpdateDNSZoneRecordsRequest{
		DNSZone: domainName,
		Changes: []*domain.RecordChange{
			{
				Delete: &domain.RecordChangeDelete{
					ID: &tracker.ID,
				},
			},
		},
	}
	_, err := c.DomainService().UpdateDNSZoneRecords(recordDeleteRequest)
	if err != nil {
		return fmt.Errorf("failed to delete record %s: %s", tracker.Name, err)
	}
	return nil
}

func deleteGateway(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	zone := scw.Zone(c.Zone())
	gwService := c.GatewayService()

	// We look for gateway connexions to private networks and detach them before deleting the gateway
	connexions, err := c.GetClusterGatewayNetworks(tracker.ID)
	if err != nil {
		return err
	}
	for _, connexion := range connexions {
		err := gwService.DeleteGatewayNetwork(&vpcgw.DeleteGatewayNetworkRequest{
			Zone:             zone,
			GatewayNetworkID: connexion.ID,
			CleanupDHCP:      true,
		})
		if err != nil {
			return fmt.Errorf("failed to detach gateway %s from private network: %s", tracker.ID, err)
		}
	}

	// We delete the gateway once it's in a stable state
	_, err = gwService.WaitForGateway(&vpcgw.WaitForGatewayRequest{
		GatewayID: tracker.ID,
		Zone:      zone,
	})
	if err != nil {
		return fmt.Errorf("error waiting for gateway: %v", err)
	}
	err = gwService.DeleteGateway(&vpcgw.DeleteGatewayRequest{
		Zone:        zone,
		GatewayID:   tracker.ID,
		CleanupDHCP: true,
	})
	if err != nil {
		return fmt.Errorf("failed to delete gateway %s: %s", tracker.ID, err)
	}

	return nil
}

func deleteLoadBalancer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	err := c.LBService().DeleteLB(&lb.DeleteLBRequest{
		Region: scw.Region(c.Region()),
		LBID:   tracker.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete load-balancer %s: %s", tracker.ID, err)
	}
	return nil
}

func deleteServer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	zone := scw.Zone(c.Zone())
	instanceService := c.InstanceService()

	srv, err := instanceService.GetServer(&instance.GetServerRequest{
		Zone:     zone,
		ServerID: tracker.ID,
	})
	if err != nil {
		klog.V(4).Infof("instance %s was already deleted", tracker.Name)
		return nil
	}

	// We detach the private network
	if len(srv.Server.PrivateNics) > 0 {
		err = instanceService.DeletePrivateNIC(&instance.DeletePrivateNICRequest{
			Zone:         zone,
			ServerID:     tracker.ID,
			PrivateNicID: srv.Server.PrivateNics[0].ID,
		})
		if err != nil {
			return fmt.Errorf("delete instance %s: error detaching private network: %v", tracker.ID, err)
		}
	}

	// If instance is running, we turn it off
	if srv.Server.State == "running" {
		_, err := instanceService.ServerAction(&instance.ServerActionRequest{
			Zone:     zone,
			ServerID: tracker.ID,
			Action:   "poweroff",
		})
		if err != nil {
			return fmt.Errorf("delete instance %s: error powering off instance: %v", tracker.ID, err)
		}
	}

	_, err = scaleway.WaitForInstanceServer(instanceService, zone, tracker.ID)
	if err != nil {
		return fmt.Errorf("delete instance %s: error waiting for instance after power-off: %v", tracker.ID, err)
	}

	err = instanceService.DeleteServer(&instance.DeleteServerRequest{
		ServerID: tracker.ID,
		Zone:     zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete server %s: %s", tracker.ID, err)
	}

	return nil
}

func deleteVolume(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	err := c.InstanceService().DeleteVolume(&instance.DeleteVolumeRequest{
		VolumeID: tracker.ID,
		Zone:     scw.Zone(c.Zone()),
	})
	if err != nil {
		return fmt.Errorf("failed to delete volume %s: %s", tracker.ID, err)
	}
	return nil
}

func deleteVPC(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	err := c.VPCService().DeletePrivateNetwork(&vpc.DeletePrivateNetworkRequest{
		PrivateNetworkID: tracker.ID,
		Zone:             scw.Zone(c.Zone()),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC %s: %s", tracker.ID, err)
	}
	return nil
}
