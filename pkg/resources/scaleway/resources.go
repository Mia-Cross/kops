package scaleway

import (
	"fmt"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"strings"

	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const (
	resourceTypeDNSRecord    = "dns-record"
	resourceTypeLoadBalancer = "load-balancer"
	resourceTypeVolume       = "volume"
	resourceTypeServer       = "server"

	KopsDomainName = "scaleway-terraform.com" //TODO: replace with real domain name later
)

type listFn func(fi.Cloud, string) ([]*resources.Resource, error)

func ListResources(cloud scaleway.ScwCloud, clusterName string) (map[string]*resources.Resource, error) {
	resourceTrackers := make(map[string]*resources.Resource)

	listFunctions := []listFn{
		listDNSRecords,
		listLoadBalancers,
		listServers,
		listVolumes,
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
	records, err := c.DomainService().ListDNSZoneRecords(&domain.ListDNSZoneRecordsRequest{
		DNSZone: KopsDomainName,
		//Type: "A", // DO only looks for records of type A, is it the same for us ?
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %s", err)
	}

	//domainName := ""
	//for _, domain := range records.Records {
	//	if strings.HasSuffix(, domain.Name()) {
	//		domainName = domain.Name()
	//	}
	//}
	//
	//if domainName == "" {
	//	if strings.HasSuffix(clusterName, ".k8s.local") {
	//		klog.Info("Domain Name is empty. Ok to have an empty domain name since cluster is configured as gossip cluster.")
	//		return nil, nil
	//	}
	//	return nil, fmt.Errorf("failed to find domain for cluster: %s", clusterName)
	//}

	resourceTrackers := []*resources.Resource(nil)
	for _, record := range records.Records {
		shortName := strings.TrimSuffix(clusterName, "."+KopsDomainName)
		if !strings.HasSuffix(record.Name, shortName) {
			continue
		}
		resourceTracker := &resources.Resource{
			Name: record.Name,
			ID:   record.ID,
			Type: resourceTypeDNSRecord,
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteRecord(cloud, tracker)
			},
			Obj: record,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listLoadBalancers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	loadBalancerName := "api-" + strings.Replace(clusterName, ".", "-", -1)

	lbs, err := c.LBService().ListLBs(&lb.ListLBsRequest{
		Region: scw.Region(c.Region()),
		Name:   &loadBalancerName,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list load-balancers: %s", err)
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, lb := range lbs.LBs {
		resourceTracker := &resources.Resource{
			Name: lb.Name,
			ID:   lb.ID,
			Type: resourceTypeLoadBalancer,
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteLoadBalancer(cloud, tracker)
			},
			Obj: lb,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listServers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)

	servers, err := c.InstanceService().ListServers(&instance.ListServersRequest{
		Zone: scw.Zone(c.Zone()),
		Tags: []string{scaleway.TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %s", err)
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, server := range servers.Servers {
		resourceTracker := &resources.Resource{
			Name:   server.Name,
			ID:     server.ID,
			Type:   resourceTypeServer,
			Blocks: []string{resourceTypeVolume},
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

	volumes, err := c.InstanceService().ListVolumes(&instance.ListVolumesRequest{
		Zone: scw.Zone(c.Zone()),
		Tags: []string{scaleway.TagClusterName + "=" + clusterName},
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err)
	}

	resourceTrackers := []*resources.Resource(nil)
	for _, volume := range volumes.Volumes {
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

func deleteRecord(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)

	recordDeleteRequest := &domain.UpdateDNSZoneRecordsRequest{
		DNSZone: KopsDomainName,
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

func deleteLoadBalancer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	lbService := c.LBService()

	err := lbService.DeleteLB(&lb.DeleteLBRequest{
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
	}
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
	zone := scw.Zone(c.Zone())
	instanceService := c.InstanceService()

	err := instanceService.DeleteVolume(&instance.DeleteVolumeRequest{
		VolumeID: tracker.ID,
		Zone:     zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete volume %s: %s", tracker.ID, err)
	}

	return nil
}
