package scalewaymodel

import (
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi"
)

// InstanceModelBuilder configures instances for the cluster
type InstanceModelBuilder struct {
	*ScwModelContext

	BootstrapScriptBuilder *model.BootstrapScriptBuilder
	Lifecycle              fi.Lifecycle
}

var _ fi.ModelBuilder = &InstanceModelBuilder{}

func (d *InstanceModelBuilder) Build(c *fi.ModelBuilderContext) error {
	//for _, ig := range d.InstanceGroups {
	//	name := d.AutoscalingGroupName(ig)
	//
	//	var instance scalewaytasks.Instance
	//	instance.Count = int(fi.Int32Value(ig.Spec.MinSize))
	//	instance.Name = fi.String(name)
	//
	//	// during alpha support we only allow 1 region
	//	// validation for only 1 region is done at this point
	//	instance.Zone = fi.String(d.Cluster.Spec.Subnets[0].Zone)
	//	instance.CommercialType = fi.String(ig.Spec.MachineType)
	//	instance.Image = fi.String(ig.Spec.Image)
	//	instance.Tags = []string{"instance-group="+ig.Name} //TODO(jtherin): find a better way
	//
	//	userData, err := d.BootstrapScriptBuilder.ResourceNodeUp(c, ig)
	//	if err != nil {
	//
	//		return err
	//	}
	//	instance.UserData = userData
	//
	//	c.AddTask(&instance)
	//}
	return nil
}
