package scaleway

import (
	"fmt"
	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"strings"

	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
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
		//Name:    "kops", // trim from cluster name ?
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
		if !strings.HasSuffix(record.Name, "kops") {
			//if !strings.HasSuffix(dns.EnsureDotSuffix(record.Name)+KopsDomainName, clusterName) {
			continue
		}
		resourceTracker := &resources.Resource{
			Name: record.Name,
			Type: resourceTypeDNSRecord,
			ID:   record.ID,
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
	//TODO: implement this function

	//c := cloud.(scaleway.ScwCloud)
	resourcesTrackers := []*resources.Resource(nil)

	return resourcesTrackers, nil
}

func listServers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	c := cloud.(scaleway.ScwCloud)
	servers, err := c.InstanceService().ListServers(&instance.ListServersRequest{
		Zone: scw.Zone(c.Zone()),
		// TODO: search by tags, it only works now because i have a single cluster and the organization does not have other instances running for other purposes but it will be problematic
		//Tags: clusterName
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %s", err)
	}

	resourcesTrackers := []*resources.Resource(nil)
	for _, server := range servers.Servers {
		resourceTracker := &resources.Resource{
			Name: server.Name,
			Type: resourceTypeServer,
			ID:   server.ID,
			Deleter: func(cloud fi.Cloud, tracker *resources.Resource) error {
				return deleteServer(cloud, tracker)
			},
			Obj: server,
		}
		resourcesTrackers = append(resourcesTrackers, resourceTracker)
	}

	return resourcesTrackers, nil
}

func listVolumes(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	//TODO: implement this function

	//c := cloud.(scaleway.ScwCloud)
	resourcesTrackers := []*resources.Resource(nil)

	return resourcesTrackers, nil
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

func deleteServer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	zone := scw.Zone(c.Zone())
	instanceService := c.InstanceService()

	_, err := instanceService.ServerAction(&instance.ServerActionRequest{
		Zone:     zone,
		ServerID: tracker.ID,
		Action:   "poweroff",
	})
	if err != nil {
		return fmt.Errorf("delete instance %s: error powering off instance: %v", tracker.ID, err)
	}

	_, err = scaleway.WaitForInstanceServer(instanceService, zone, tracker.ID)
	if err != nil {
		return fmt.Errorf("delete instance %s: error waiting for instance: %v", tracker.ID, err)
	}

	err = c.InstanceService().DeleteServer(&instance.DeleteServerRequest{
		ServerID: tracker.ID,
		Zone:     zone,
	})
	if err != nil {
		return fmt.Errorf("failed to delete server %s: %s", tracker.ID, err)
	}

	return nil
}
