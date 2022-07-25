package scaleway

import (
	"fmt"
	"k8s.io/kops/dns-controller/pkg/dns"
	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"strings"

	domain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const (
	resourceTypeDNSRecord    = "dns-record"
	resourceTypeLoadBalancer = "load-balancer"
	resourceTypeVolume       = "volume"

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
		Name:    "kops", // trim from cluster name ?
		//Type: "A", // DO only looks for records of type A, is it the same for us ?
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %s", err)
	}

	//domainName := ""
	//for _, domain := range domains {
	//	if strings.HasSuffix(clusterName, domain.Name()) {
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
		if !strings.HasSuffix(dns.EnsureDotSuffix(record.Name)+KopsDomainName, clusterName) {
			continue
		}
		resourceTracker := &resources.Resource{
			Name: record.Name,
			Type: resourceTypeDNSRecord,
			ID:   record.ID,
			Deleter: func(cloud fi.Cloud, resourceTracker *resources.Resource) error {
				return deleteRecord(cloud, KopsDomainName, resourceTracker)
			},
			Obj: record,
		}
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func listLoadBalancers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	//TODO: implement this function

	//c := cloud.(scw.ScwCloud)
	resourcesTrackers := []*resources.Resource(nil)

	return resourcesTrackers, nil
}

func listServers(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	//TODO: implement this function

	//c := cloud.(scw.ScwCloud)
	resourcesTrackers := []*resources.Resource(nil)

	return resourcesTrackers, nil
}

func listVolumes(cloud fi.Cloud, clusterName string) ([]*resources.Resource, error) {
	//TODO: implement this function

	//c := cloud.(scw.ScwCloud)
	resourcesTrackers := []*resources.Resource(nil)

	return resourcesTrackers, nil
}

func deleteRecord(cloud fi.Cloud, name string, tracker *resources.Resource) error {
	c := cloud.(scaleway.ScwCloud)
	//domainName := "scaleway-terraform.com"
	_, err := c.DomainService().DeleteDNSZone(&domain.DeleteDNSZoneRequest{
		DNSZone: name,
		//DNSZone:   name+"."+domainName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete record %s: %s", name, err)
	}
	return nil
}
