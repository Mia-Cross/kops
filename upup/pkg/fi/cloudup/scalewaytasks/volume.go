package scalewaytasks

import (
	"fmt"
	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// +kops:fitask
type Volume struct {
	Name      *string
	ID        *string
	Lifecycle fi.Lifecycle

	Size *int64
	//Region *string
	Zone *string
	Tags map[string]string
}

var _ fi.CompareWithID = &Volume{}

func (v *Volume) CompareWithID() *string {
	return v.ID
}

func (v *Volume) Find(c *fi.Context) (*Volume, error) {
	cloud := c.Cloud.(scaleway.ScwCloud)
	instanceService := cloud.InstanceService()
	zone := cloud.Zone()

	volumes, err := instanceService.ListVolumes(&instance.ListVolumesRequest{
		Name: v.Name,
		Zone: scw.Zone(zone),
	}, scw.WithAllPages())
	if err != nil {
		return nil, err
	}

	for _, volume := range volumes.Volumes {
		if volume.Name == fi.StringValue(v.Name) {
			return &Volume{
				Name:      fi.String(volume.Name),
				ID:        fi.String(volume.ID),
				Lifecycle: v.Lifecycle,
				Size:      fi.Int64(int64(volume.Size)),
				Zone:      fi.String(string(volume.Zone)),
			}, nil
		}
	}

	return nil, nil
}

func (v *Volume) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(v, c)
}

func (_ *Volume) CheckChanges(a, e, changes *Volume) error {
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
		if e.Size == nil {
			return fi.RequiredField("Size")
		}
		if e.Zone == nil {
			return fi.RequiredField("Zone")
		}
	}
	return nil
}

func (_ *Volume) RenderScw(t *scaleway.ScwAPITarget, a, e, changes *Volume) error {
	if a != nil {
		// in general, we shouldn't need to render changes to a volume
		// however there can be cases where we may want to resize or rename.
		// consider this in later stages of Scw support on kops
		return nil
	}

	tagArray := []string{}

	for k, v := range e.Tags {
		// Scw tags don't accept =. Separate the key and value with an ":"
		klog.V(10).Infof("Scw - Join the volume tag - %s", fmt.Sprintf("%s:%s", k, v))
		tagArray = append(tagArray, fmt.Sprintf("%s:%s", k, v))
	}

	instanceService := t.Cloud.InstanceService()
	_, err := instanceService.CreateVolume(&instance.CreateVolumeRequest{
		Zone: scw.Zone(fi.StringValue(e.Zone)),
		Name: fi.StringValue(e.Name),
		Size: scw.SizePtr(scw.Size(fi.Int64Value(e.Size))),
		Tags: tagArray,
	})

	return err
}

// terraformVolume represents the scaleway_instance_volume resource in terraform
// https://registry.terraform.io/providers/scaleway/scaleway/latest/docs/resources/instance_volume
type terraformVolume struct {
	Name   *string `cty:"name"`
	SizeGB *int64  `cty:"size"`
	Zone   *string `cty:"zone"`
}

func (_ *Volume) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *Volume) error {
	sizeGB := fi.Int64Value(e.Size) / 1e9
	tf := &terraformVolume{
		Name:   e.Name,
		SizeGB: &sizeGB,
		Zone:   e.Zone,
	}
	return t.RenderResource("scaleway_instance_volume", *e.Name, tf)
}
