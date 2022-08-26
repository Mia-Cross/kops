echo "--> DNS RECORDS :"
scw dns record list scaleway-terraform.com -p devterraform
echo "\n--> LOAD - BALANCERS :"
scw lb lb list zone=fr-par-1 -p devterraform
echo "\n--> SERVERS :"
scw instance server list zone=fr-par-2 -p devterraform
echo "\n--> VOLUMES :"
scw instance volume list zone=fr-par-2 -p devterraform

#--> SERVERS :
#ID                                    NAME                                                 TYPE    STATE    ZONE      PUBLIC IP       PRIVATE IP
#559ac801-b002-4a43-a3ce-ed2715789e98  nodes-fr-par-2.kops.scaleway-terraform.com           DEV1-M  running  fr-par-2  51.159.188.80   10.197.124.3
#8f51f1fb-4f23-421d-9536-d324874021ac  master-fr-par-2.masters.kops.scaleway-terraform.com  DEV1-M  running  fr-par-2  51.159.131.102  10.197.146.73
#
#--> VOLUMES :
#ID                                    STATE      SERVER ID                             SERVER NAME                                          NAME                                            PROJECT
#41560df4-1390-49cb-8bbd-7e5dd8a36da0  available  559ac801-b002-4a43-a3ce-ed2715789e98  nodes-fr-par-2.kops.scaleway-terraform.com           debian_buster:volume-0                          105bdce1-64c0-48ab-899d-868455867ecf
#dcf8b7dd-e89f-4b66-8c79-66bb2a081cbe  available  8f51f1fb-4f23-421d-9536-d324874021ac  master-fr-par-2.masters.kops.scaleway-terraform.com  debian_buster:volume-0                          105bdce1-64c0-48ab-899d-868455867ecf
#dbc2cc48-425c-4c36-828c-4150add8452b  available                                                                                             etcd-2.etcd-events.kops.scaleway-terraform.com  105bdce1-64c0-48ab-899d-868455867ecf
#6628f294-d75b-47d9-b25f-f49386cffcdd  available                                                                                             etcd-2.etcd-main.kops.scaleway-terraform.com    105bdce1-64c0-48ab-899d-868455867ecf

