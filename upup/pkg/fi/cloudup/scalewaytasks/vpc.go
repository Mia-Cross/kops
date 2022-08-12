package scalewaytasks

import (
	"fmt"
	"os"

	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
)

// +kops:fitask
type VPC struct {
	Name      *string
	ID        *string
	Lifecycle fi.Lifecycle
	IPRange   *string
	Zone      *string
}

var _ fi.CompareWithID = &VPC{}

func (v *VPC) CompareWithID() *string {
	return v.ID
}

func (v *VPC) Find(c *fi.Context) (*VPC, error) {
	cloud := c.Cloud.(scaleway.ScwCloud)
	vpcService := cloud.VPCService()

	vpcs, err := vpcService.ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
		Zone: scw.Zone(cloud.Zone()),
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("error listing instances: %s", err)
	}

	for _, vpc := range vpcs.PrivateNetworks {
		if vpc.Name == fi.StringValue(v.Name) {
			subnet := ""
			if len(vpc.Subnets) > 0 {
				subnet = vpc.Subnets[0].String()
			}
			return &VPC{
				Name:      fi.String(vpc.Name),
				ID:        fi.String(vpc.ID),
				Lifecycle: v.Lifecycle,
				IPRange:   &subnet,
				Zone:      fi.String(string(vpc.Zone)),
			}, nil
		}
	}
	return nil, nil
}

func (v *VPC) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(v, c)
}

func (_ *VPC) CheckChanges(a, e, changes *VPC) error {
	if a != nil {
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
		if changes.ID != nil {
			return fi.CannotChangeField("ID")
		}
		if changes.Zone != nil {
			return fi.CannotChangeField("Zone")
		}
	} else {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		if e.Zone == nil {
			return fi.RequiredField("Zone")
		}
	}
	return nil
}

func (_ *VPC) RenderScw(t *scaleway.ScwAPITarget, a, e, changes *VPC) error {
	if a != nil {
		return nil
	}

	vpcService := t.Cloud.VPCService()
	_, err := vpcService.CreatePrivateNetwork(&vpc.CreatePrivateNetworkRequest{
		Zone:      scw.Zone(fi.StringValue(e.Zone)),
		Name:      fi.StringValue(e.Name),
		ProjectID: os.Getenv("SCW_DEFAULT_PROJECT_ID"),
	})
	if err != nil {
		return fmt.Errorf("error rendering VPC: %s", err)
	}

	return nil
}
