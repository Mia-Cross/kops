package scalewaymodel

import (
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
	"k8s.io/kops/upup/pkg/fi/cloudup/scalewaytasks"
)

// InstanceModelBuilder configures instances for the cluster
type InstanceModelBuilder struct {
	*ScwModelContext

	BootstrapScriptBuilder *model.BootstrapScriptBuilder
	Lifecycle              fi.Lifecycle
}

var _ fi.ModelBuilder = &InstanceModelBuilder{}

func (d *InstanceModelBuilder) Build(c *fi.ModelBuilderContext) error {
	for _, ig := range d.InstanceGroups {
		name := d.AutoscalingGroupName(ig)

		instance := scalewaytasks.Instance{
			Count:     int(fi.Int32Value(ig.Spec.MinSize)),
			Name:      fi.String(name),
			Lifecycle: d.Lifecycle,
			Network:   d.LinkToNetwork(),

			// during alpha support we only allow 1 region
			// validation for only 1 region is done at this point
			Zone:           fi.String(d.Cluster.Spec.Subnets[0].Zone),
			CommercialType: fi.String(ig.Spec.MachineType),
			Image:          fi.String(ig.Spec.Image),
			Tags: []string{
				scaleway.TagInstanceGroup + "=" + ig.Name, //TODO(jtherin): find a better way
				scaleway.TagClusterName + "=" + d.Cluster.Name,
			},
		}

		userData, err := d.BootstrapScriptBuilder.ResourceNodeUp(c, ig)
		if err != nil {
			return err
		}
		instance.UserData = &userData

		c.AddTask(&instance)
	}
	return nil
}
