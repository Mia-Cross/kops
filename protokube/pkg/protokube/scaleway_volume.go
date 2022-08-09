package protokube

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/klog/v2"
	kopsv "k8s.io/kops"
	"k8s.io/kops/protokube/pkg/gossip"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
)

const (
	apiMetadataURL = "http://169.254.42.42/user_data"
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
	scwClient, err := scw.NewClient(
		scw.WithUserAgent("kubernetes-kops/"+kopsv.Version),
		scw.WithEnv(),
	)
	if err != nil {
		return nil, err
	}

	//instanceAPI := instance.NewAPI(scwClient)

	userData, err := getScwMetadata(apiMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve server id: %s", err)
	}
	klog.V(4).Infof("Found ID of the running server: %d", userData)

	//server, err := GetInstanceServer(&instance.GetServerRequest{
	//	ServerID: serverID,
	//	Zone: zone,
	//})
	//if err != nil || server == nil {
	//	return nil, fmt.Errorf("failed to get info for the running server: %s", err)
	//}
	//klog.V(4).Infof("Found name of the running server: %q", server.Name)
	//
	//if len(server.) > 0 {
	//	klog.V(4).Infof("Found first private net IP of the running server: %q", server.PrivateNet[0].IP.String())
	//} else {
	//	return nil, fmt.Errorf("failed to find private net of the running server")
	//}

	s := &ScwCloudProvider{
		scwClient: scwClient,
		server:    nil,
		serverIP:  nil,
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

func getScwMetadata(url string) (string, error) { //TODO(Mia-Cross): change this a bit
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("droplet metadata returned non-200 status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
