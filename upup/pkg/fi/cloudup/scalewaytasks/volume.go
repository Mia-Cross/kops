package scalewaytasks

import (
	"fmt"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"

	"github.com/digitalocean/godo"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// +kops:fitask
type Volume struct {
	Name      *string
	ID        *string
	Lifecycle fi.Lifecycle

	SizeGB int
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

	name := fi.StringValue(v.Name)
	volumes, err := instanceService.ListVolumes(&instance.ListVolumesRequest{
		Name: &name,
		Zone: cloud.Zone,
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
				SizeGB:    fi.Int64(volume.Size), // TODO: convert bytes result to GBytes
				Region:    fi.String(volume.Region.Slug),
			}, nil
		}
	}

	// Volume = nil if not found
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
		if changes.Region != nil {
			return fi.CannotChangeField("Region")
		}
	} else {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		if e.SizeGB == nil {
			return fi.RequiredField("SizeGB")
		}
		if e.Region == nil {
			return fi.RequiredField("Region")
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

	volService := t.Cloud.VolumeService()
	_, _, err := volService.CreateVolume(context.TOScw(), &godo.VolumeCreateRequest{
		Name:          fi.StringValue(e.Name),
		Region:        fi.StringValue(e.Region),
		SizeGigaBytes: fi.Int64Value(e.SizeGB),
		Tags:          tagArray,
	})

	return err
}

// terraformVolume represents the digitalocean_volume resource in terraform
// https://www.terraform.io/docs/providers/scaleway/r/volume.html
type terraformVolume struct {
	Name   *string `cty:"name"`
	SizeGB *int64  `cty:"size"`
	Region *string `cty:"region"`
}

func (_ *Volume) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *Volume) error {
	tf := &terraformVolume{
		Name:   e.Name,
		SizeGB: e.SizeGB,
		Region: e.Region,
	}
	return t.RenderResource("digitalocean_volume", *e.Name, tf)
}
