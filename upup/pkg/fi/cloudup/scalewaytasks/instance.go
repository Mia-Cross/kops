package scalewaytasks

import (
	"bytes"
	"fmt"
	"os"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	_ "k8s.io/kops/upup/pkg/fi/cloudup/terraform"
)

// +kops:fitask
type Instance struct {
	Name      *string
	Lifecycle fi.Lifecycle

	Zone           *string
	CommercialType *string
	Image          *string
	Tags           []string
	Count          int
	UserData       *fi.Resource
	//Network        *Network
	NeedsUpdate bool
}

var _ fi.Task = &Instance{}
var _ fi.CompareWithID = &Instance{}

func (s *Instance) CompareWithID() *string {
	return s.Name
}

func (s *Instance) Find(c *fi.Context) (*Instance, error) {
	cloud := c.Cloud.(scaleway.ScwCloud)

	servers, err := cloud.GetClusterServers(cloud.ClusterName(s.Tags), s.Name)
	if err != nil || len(servers) == 0 {
		return nil, err
	}

	//for i, server := range responseServers.Servers {
	// Check if servers have been added to the instance group, therefore an update is needed
	//if len(servers) >= s.Count {
	//	s.NeedsUpdate = true
	//	//continue
	//}
	//}
	//TODO(Mia-Cross): we also need to find a way to tag existing servers as needing an update if etcd-clusters were added
	server := servers[0]

	return &Instance{
		Name:           fi.String(server.Name),
		Count:          len(servers),
		Zone:           fi.String(server.Zone.String()),
		CommercialType: fi.String(server.CommercialType),
		Image:          s.Image, // image label is lost by server api
		Tags:           server.Tags,
		UserData:       s.UserData, // TODO(Mia-Cross): get from instance or ignore change
		Lifecycle:      s.Lifecycle,
		//Network:        s.Network,
	}, nil
}

func (s *Instance) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(s, c)
}

func (_ *Instance) RenderScw(c *fi.Context, a, e, changes *Instance) error {
	cloud := c.Cloud.(scaleway.ScwCloud)
	instanceService := cloud.InstanceService()
	zone := scw.Zone(fi.StringValue(e.Zone))

	//if a != nil {
	//	// Add "kops.k8s.io/needs-update" label to servers needing update
	//	if a.NeedsUpdate == true {
	//		servers, err := cloud.GetClusterServers(cloud.ClusterName(), a.Name)
	//		if err != nil {
	//			return fmt.Errorf("error rendering server group: error listing existing servers: %w", err)
	//		}
	//		for _, server := range servers {
	//			_, err = instanceService.UpdateServer(&instance.UpdateServerRequest{
	//				Zone:     zone,
	//				ServerID: server.ID,
	//				Tags:     scw.StringsPtr(append(server.Tags, scaleway.TagNeedsUpdate)),
	//			})
	//			if err != nil {
	//				return fmt.Errorf("error rendering server group: error adding update tag to server %q (%s): %w", server.Name, server.ID, err)
	//			}
	//		}
	//	}
	//}

	userData, err := fi.ResourceAsBytes(*e.UserData)
	if err != nil {
		return err
	}

	var newInstanceCount int
	if a == nil {
		newInstanceCount = e.Count
	} else {
		expectedCount := e.Count
		actualCount := a.Count

		if expectedCount == actualCount {
			return nil
		}

		if actualCount > expectedCount {
			igInstances, err := cloud.GetClusterServers(cloud.ClusterName(a.Tags), a.Name)
			if err != nil {
				return fmt.Errorf("error listing instances named %s", fi.StringValue(a.Name))
			}
			for _, igInstance := range igInstances {
				err = cloud.DeleteServer(igInstance)
				if err != nil {
					return fmt.Errorf("error deleting instance %s of group %s", igInstance.ID, igInstance.Name)
				}
				actualCount--
				if expectedCount == actualCount {
					break
				}
			}
		}

		newInstanceCount = expectedCount - actualCount
	}

	//pn, err := cloud.GetClusterVPCs(c.Cluster.Name)
	//if err != nil {
	//	return fmt.Errorf("error listing private networks: %v", err)
	//}
	//if len(pn) != 1 {
	//	return fmt.Errorf("more than 1 private network named %s found", c.Cluster.Name)
	//}

	mastersPrivateIPs := []string(nil)

	for i := 0; i < newInstanceCount; i++ {

		// We create the instance
		srv, err := instanceService.CreateServer(&instance.CreateServerRequest{
			Zone:           zone,
			Name:           fi.StringValue(e.Name),
			CommercialType: fi.StringValue(e.CommercialType),
			Image:          fi.StringValue(e.Image),
			Tags:           e.Tags,
		})
		if err != nil {
			return fmt.Errorf("error creating instance with name %s: %s", fi.StringValue(e.Name), err)
		}

		// We wait for the instance to be ready
		_, err = scaleway.WaitForInstanceServer(instanceService, zone, srv.Server.ID)
		if err != nil {
			return fmt.Errorf("error waiting for instance with name %s: %s", fi.StringValue(e.Name), err)
		}

		// We load the cloud-init script in the instance user data
		err = instanceService.SetServerUserData(&instance.SetServerUserDataRequest{
			ServerID: srv.Server.ID,
			Zone:     srv.Server.Zone,
			Key:      "cloud-init",
			Content:  bytes.NewBuffer(userData),
		})
		if err != nil {
			return fmt.Errorf("error setting 'cloud-init' in user-data: %s", err)
		}

		// We start the instance
		_, err = instanceService.ServerAction(&instance.ServerActionRequest{
			Zone:     zone,
			ServerID: srv.Server.ID,
			Action:   instance.ServerActionPoweron,
		})
		if err != nil {
			return fmt.Errorf("error powering on instance with name %s: %s", fi.StringValue(e.Name), err)
		}

		// We wait for the instance to be ready
		_, err = scaleway.WaitForInstanceServer(instanceService, zone, srv.Server.ID)
		if err != nil {
			return fmt.Errorf("error waiting for instance with name %s: %s", fi.StringValue(e.Name), err)
		}

		// We update the server's infos (to get its IP)
		server, err := instanceService.GetServer(&instance.GetServerRequest{
			Zone:     zone,
			ServerID: srv.Server.ID,
		})
		if err != nil {
			return fmt.Errorf("error getting server %s: %s", srv.Server.ID, err)
		}

		// If instance has role master, we add its private IP to the list to add it to the lb's backend
		for _, tag := range e.Tags {
			if tag == scaleway.TagNameRolePrefix+scaleway.TagRoleMaster {
				mastersPrivateIPs = append(mastersPrivateIPs, *server.Server.PrivateIP)
			}
		}

		// We put the instance inside the private network
		//pNIC, err := instanceService.CreatePrivateNIC(&instance.CreatePrivateNICRequest{
		//	Zone:             zone,
		//	ServerID:         srv.Server.ID,
		//	PrivateNetworkID: pn[0].ID,
		//})
		//if err != nil {
		//	return fmt.Errorf("error linking instance to private network: %v", err)
		//}
		//
		//// We wait for the private nic to be ready before proceeding
		//_, err = instanceService.WaitForPrivateNIC(&instance.WaitForPrivateNICRequest{
		//	ServerID:     srv.Server.ID,
		//	PrivateNicID: pNIC.PrivateNic.ID,
		//	Zone:         zone,
		//})
		//if err != nil {
		//	return fmt.Errorf("error waiting for private nic: %v", err)
		//}
	}

	// If IG is master, we add the new servers' IPs to the load-balancer's back-end
	if len(mastersPrivateIPs) > 0 {
		lbService := cloud.LBService()
		region := scw.Region(os.Getenv("SCW_DEFAULT_REGION"))

		lbs, err := cloud.GetClusterLoadBalancers(cloud.ClusterName(e.Tags))
		if err != nil {
			return fmt.Errorf("error listing load-balancers for instance creation: %w", err)
		}

		for _, loadBalancer := range lbs {
			backEnds, err := lbService.ListBackends(&lb.ListBackendsRequest{
				Region: region,
				LBID:   loadBalancer.ID,
			})
			if err != nil {
				return fmt.Errorf("error listing load-balancer's back-ends for instance creation: %w", err)
			}
			if backEnds.TotalCount > 1 {
				return fmt.Errorf("found multiple back-ends for load-balancer %s, exiting now", loadBalancer.ID)
			}
			backEnd := backEnds.Backends[0]

			_, err = lbService.AddBackendServers(&lb.AddBackendServersRequest{
				Region:    region,
				BackendID: backEnd.ID,
				ServerIP:  mastersPrivateIPs,
			})
			if err != nil {
				return fmt.Errorf("error adding servers' IPs to load-balancer's back-end: %w", err)
			}

			_, err = lbService.WaitForLb(&lb.WaitForLBRequest{
				LBID:   loadBalancer.ID,
				Region: region,
			})
			if err != nil {
				return fmt.Errorf("error waiting for load-balancer %s: %w", loadBalancer.ID, err)
			}
		}
	}

	//gwService := cloud.GatewayService()
	//rules := []*vpcgw.SetPATRulesRequestRule(nil)
	//port := uint32(2022)

	// We create NAT rules linking the gateway to our instances in order to be able to connect via SSH
	//// TODO(Mia-Cross): This part is for dev purposes only, remove when done
	//gwNetwork, err := cloud.GetClusterGatewayNetworks(pn[0].ID)
	//if err != nil {
	//	return err
	//}
	//if len(gwNetwork) < 1 {
	//	klog.V(4).Infof("Could not find any gateway connexion, skipping NAT rules creation")
	//} else {
	//	entries, err := gwService.ListDHCPEntries(&vpcgw.ListDHCPEntriesRequest{
	//		Zone:             zone,
	//		GatewayNetworkID: scw.StringPtr(gwNetwork[0].ID),
	//	}, scw.WithAllPages())
	//	if err != nil {
	//		return fmt.Errorf("error listing DHCP entries")
	//	}
	//	klog.V(4).Infof("=== DHCP entries are %v", entries.DHCPEntries)
	//	for _, entry := range entries.DHCPEntries {
	//		rules = append(rules, &vpcgw.SetPATRulesRequestRule{
	//			PublicPort:  port,
	//			PrivateIP:   entry.IPAddress,
	//			PrivatePort: 22,
	//			Protocol:    "both",
	//		})
	//		port += 1
	//	}
	//
	//	_, err = gwService.SetPATRules(&vpcgw.SetPATRulesRequest{
	//		Zone:      zone,
	//		GatewayID: gwNetwork[0].GatewayID,
	//		PatRules:  rules,
	//	})
	//	if err != nil {
	//		return fmt.Errorf("error setting PAT rules for gateway")
	//	}
	//	klog.V(4).Infof("=== rules set")
	//}
	return nil
}

func (_ *Instance) CheckChanges(a, e, changes *Instance) error {
	if a != nil {
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
		if changes.Zone != nil {
			return fi.CannotChangeField("Zone")
		}
		if changes.CommercialType != nil {
			return fi.CannotChangeField("CommercialType")
		}
		if changes.Image != nil {
			return fi.CannotChangeField("Image")
		}
	} else {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		if e.Zone == nil {
			return fi.RequiredField("Zone")
		}
		if e.CommercialType == nil {
			return fi.RequiredField("CommercialType")
		}
		if e.Image == nil {
			return fi.RequiredField("Image")
		}
	}
	return nil
}
