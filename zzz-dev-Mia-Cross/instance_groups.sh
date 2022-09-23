###########################################
#         cluster.leila.sieben.fr         #
###########################################

# NODE
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.leila.sieben.fr extra-node --role node
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.leila.sieben.fr extra-node

# MASTER
go run -v ./cmd/kops get cluster -o yaml > mycluster.yaml
go run -v ./cmd/kops replace -f mycluster.yaml
#go run -v ./cmd/kops/ edit cluster -v10 --name=cluster.leila.sieben.fr
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.leila.sieben.fr extra-master --role master --subnet fr-par-1
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.leila.sieben.fr extra-master2 --role master --subnet fr-par-1
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.leila.sieben.fr extra-master
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.leila.sieben.fr extra-master2

###########################################
#            cluster.k8s.local            #
###########################################

# NODE
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.k8s.local extra-node --role node
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.k8s.local extra-node

# MASTER
go run -v ./cmd/kops get cluster -o yaml > mycluster.yaml
go run -v ./cmd/kops replace -f mycluster.yaml
#go run -v ./cmd/kops/ edit cluster -v10 --name=cluster.k8s.local
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.k8s.local extra-master --role master --subnet fr-par-1
go run -v ./cmd/kops/ create instancegroup -v10 --name=cluster.k8s.local extra-master-2 --role master --subnet fr-par-1
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.k8s.local extra-master
go run -v ./cmd/kops/ delete instancegroup -v10 --name=cluster.k8s.local extra-master-2

