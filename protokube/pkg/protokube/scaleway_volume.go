package protokube

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/klog/v2"
	kopsv "k8s.io/kops"
	"k8s.io/kops/protokube/pkg/gossip"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
)

// ScwCloudProvider defines the Scaleway Cloud volume implementation.
type ScwCloudProvider struct {
	scwClient *scw.Client
	server    *instance.Server
	serverIP  net.IP
}

var _ CloudProvider = &ScwCloudProvider{}

// NewScwCloudProvider returns a new Scaleway Cloud volume provider.
func NewScwCloudProvider() (*ScwCloudProvider, error) {
	fmt.Println("HI, YOU'RE DEALING WITH SCALEWAY VOLUMES")
	scwClient, err := scw.NewClient(
		scw.WithUserAgent("kubernetes-kops/"+kopsv.Version),
		scw.WithEnv(),
	)
	if err != nil {
		return nil, err
	}

	metadataAPI := instance.NewMetadataAPI()
	metadata, err := metadataAPI.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve server metadata: %s", err)
	}

	serverID := metadata.ID
	klog.V(4).Infof("Found ID of the running server: %v", serverID)
	zoneID := metadata.Location.ZoneID
	klog.V(4).Infof("Found zone of the running server: %v", zoneID)
	zone, err := formatZone(zoneID)
	if err != nil {
		return nil, fmt.Errorf("unable to format Scaleway zone: %s", err)
	}
	klog.V(4).Infof("Found zone of the running server: %v", zone)
	privateIP := metadata.PrivateIP
	klog.V(4).Infof("Found first private net IP of the running server: %q", privateIP)

	instanceAPI := instance.NewAPI(scwClient)
	server, err := instanceAPI.GetServer(&instance.GetServerRequest{
		ServerID: serverID,
		Zone:     zone,
	})
	if err != nil || server == nil {
		return nil, fmt.Errorf("failed to get the running server: %s", err)
	}
	klog.V(4).Infof("Found the running server: %q", server.Server.Name)

	s := &ScwCloudProvider{
		scwClient: scwClient,
		server:    server.Server,
		serverIP:  net.IP(privateIP),
	}

	return s, nil
}

func (s *ScwCloudProvider) InstanceID() string {
	return fmt.Sprintf("%s-%s", s.server.Name, s.server.ID)
}

func (s ScwCloudProvider) InstanceInternalIP() net.IP {
	return s.serverIP
}

func (s *ScwCloudProvider) GossipSeeds() (gossip.SeedProvider, error) {
	for _, tag := range s.server.Tags {
		if !strings.HasPrefix(tag, scaleway.TagClusterName) {
			continue
		}
		clusterName := strings.TrimPrefix(tag, scaleway.TagClusterName+"=")
		//return gossipscw.NewSeedProvider(s.scwClient, clusterName)
		return nil, errors.New(clusterName) // TODO(Mia-Cross): remove that !!!!
	}
	return nil, fmt.Errorf("failed to find cluster name label for running server: %v", s.server.Tags)
}

func formatZone(zoneID string) (scw.Zone, error) {
	switch zoneID {
	case "par1":
		return scw.ZoneFrPar1, nil
	case "par2":
		return scw.ZoneFrPar2, nil
	case "par3":
		return scw.ZoneFrPar3, nil
	case "ams1":
		return scw.ZoneNlAms1, nil
	case "ams2":
		return scw.ZoneNlAms2, nil
	case "waw1":
		return scw.ZonePlWaw1, nil
	default:
		return "", fmt.Errorf("unknown zone %s", zoneID)
	}
}
