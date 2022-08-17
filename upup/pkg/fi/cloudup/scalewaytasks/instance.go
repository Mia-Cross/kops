package scalewaytasks

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
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
	Network        *Network
}

var _ fi.Task = &Instance{}
var _ fi.CompareWithID = &Instance{}

func (d *Instance) CompareWithID() *string {
	return d.Name
}

func (d *Instance) Find(c *fi.Context) (*Instance, error) {
	cloud := c.Cloud.(scaleway.ScwCloud)

	listServersArgs := &instance.ListServersRequest{
		Zone: scw.Zone(fi.StringValue(d.Zone)),
		//Name:   &name,
	}

	responseServers, err := cloud.InstanceService().ListServers(listServersArgs, scw.WithAllPages())
	if err != nil {
		return nil, err
	}

	count := 0
	var lastServer *instance.Server
	for _, srv := range responseServers.Servers {
		if srv.Name == fi.StringValue(d.Name) {
			count++
			lastServer = srv
		}
	}

	if lastServer == nil {
		return nil, nil
	}

	return &Instance{
		Name:           fi.String(lastServer.Name),
		Count:          count,
		Zone:           fi.String(lastServer.Zone.String()),
		CommercialType: fi.String(lastServer.CommercialType),
		Image:          d.Image, // image label is lost by server api
		Tags:           lastServer.Tags,
		UserData:       d.UserData, // TODO(Mia-Cross): get from instance or ignore change
		Lifecycle:      d.Lifecycle,
		Network:        d.Network,
	}, nil
}

func (d *Instance) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(d, c)
}

func (_ *Instance) RenderScw(c *fi.Context, a, e, changes *Instance) error {
	cloud := c.Cloud.(scaleway.ScwCloud)

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
			return errors.New("deleting instances is not supported yet")
		}

		newInstanceCount = expectedCount - actualCount
	}

	instanceService := cloud.InstanceService()
	zone := scw.Zone(fi.StringValue(e.Zone))

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

		// We add the tags to the volume of the instance to be able to delete it later
		_, err = instanceService.UpdateVolume(&instance.UpdateVolumeRequest{
			Zone:     zone,
			VolumeID: srv.Server.Volumes["0"].ID,
			Tags:     &e.Tags,
		})
		if err != nil {
			return fmt.Errorf("error addings tags to volume for instance %s: %s", fi.StringValue(e.Name), err)
		}

		// We load the cloudinit script in the instance user data
		err = instanceService.SetServerUserData(&instance.SetServerUserDataRequest{
			ServerID: srv.Server.ID,
			Zone:     srv.Server.Zone,
			Key:      "cloud-init",
			Content:  bytes.NewBuffer(userData),
		})
		if err != nil {
			return fmt.Errorf("error setting 'cloud-init' in user-data: %s", err)
		}

		// We put the instance inside the private network
		pn, err := cloud.VPCService().ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
			Zone: zone,
			Name: scw.StringPtr(c.Cluster.Name),
			//Tags:           []string{scaleway.TagClusterName + "=" + clusterName},
		})
		if err != nil {
			return fmt.Errorf("error listing private networks: %v", err)
		}
		if pn.TotalCount != 1 {
			return fmt.Errorf("more than 1 private network named %s found", c.Cluster.Name)
		}
		_, err = instanceService.CreatePrivateNIC(&instance.CreatePrivateNICRequest{
			Zone:             zone,
			ServerID:         srv.Server.ID,
			PrivateNetworkID: pn.PrivateNetworks[0].ID,
		})
		if err != nil {
			return fmt.Errorf("error linking instance to private network: %v", err)
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

	}
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
