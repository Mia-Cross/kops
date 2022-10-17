package scaleway

import (
	"fmt"
	"strings"

	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"

	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const (
	resourceTypeDNSRecord    = "dns-record"
	resourceTypeGateway      = "gateway"
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

	if strings.HasSuffix(clusterName, ".k8s.local") {
		return nil, nil
	}

	names := strings.SplitN(clusterName, ".", 2)
	clusterNameShort := names[0]
	domainName := names[1]

	records, err := c.DomainService().ListDNSZoneRecords(&domain.ListDNSZoneRecordsRequest{
		DNSZone: domainName,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %s", err)
	}

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
	servers, err := c.GetClusterServers(clusterName, nil)
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
	record := tracker.Obj.(*domain.Record)

	return c.DeleteRecord(record, domainName)
}

func deleteGateway(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	gateway := tracker.Obj.(*vpcgw.Gateway)

	return c.DeleteGateway(gateway)
}

func deleteLoadBalancer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	loadBalancer := tracker.Obj.(*lb.LB)

	return c.DeleteLoadBalancer(loadBalancer)
}

func deleteServer(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	server := tracker.Obj.(*instance.Server)

	return c.DeleteServer(server)
}

func deleteVolume(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	volume := tracker.Obj.(*instance.Volume)

	return c.DeleteVolume(volume)
}

func deleteVPC(cloud fi.Cloud, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	privateNetwork := tracker.Obj.(*vpc.PrivateNetwork)

	return c.DeleteVPC(privateNetwork)
}
