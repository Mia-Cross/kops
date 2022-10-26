echo "--> DNS RECORDS :"
scw dns record list scaleway-terraform.com -p devterraform
echo "\n--> LOAD - BALANCERS :"
scw lb lb list zone=fr-par-1 -p devterraform
echo "\n--> SERVERS :"
scw instance server list zone=fr-par-2 -p devterraform
echo "\n--> VOLUMES :"
scw instance volume list zone=fr-par-2 -p devterraform                                                                                        etcd-2.etcd-main.kops.scaleway-terraform.com    105bdce1-64c0-48ab-899d-868455867ecf

