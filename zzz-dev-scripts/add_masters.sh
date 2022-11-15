CLUSTER_NAME=$1
SPEC_FILES_DIR=zzz-dev-scripts

go run -v ./cmd/kops replace -f "$SPEC_FILES_DIR/$CLUSTER_NAME"_extra_masters.yaml
go run -v ./cmd/kops/ create instancegroup -v10 --name=$CLUSTER_NAME master2 --role master --subnet fr-par-1 --edit=false
go run -v ./cmd/kops/ create instancegroup -v10 --name=$CLUSTER_NAME master3 --role master --subnet fr-par-1 --edit=false
#go run -v ./cmd/kops/ update cluster -v10 --name=$CLUSTER_NAME --yes