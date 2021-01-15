# iSCSI-provisioner 

iSCSI-provisioner is an out of tree provisioner for iSCSI storage for
Kubernetes and OpenShift.  The provisioniner uses the API provided by
[iscsi-target-api](https://github.com/ogre0403/iscsi-target-api) to create and export
iSCSI storage on a remote server.

## How it works

When a pvc request is issued for an iscsi provisioner controlled storage class the following happens:

1. a new disk volume is created, the size of the volume corresponds to the size requested in the pvc.

2. a new target with single LUN is created on iSCSI target server, the new target name is `iqn.<YEAR>-<MM>.k8s.<namespace>:<pvc-name>` 

3. the disk volume attach to LUN with id `1` and made accessible to ALL initiators. 
   That means, it doesn't support iSCSI ACL for now.  

4. the corresponding pv is created and bound to the pvc.


## A note about names

In various places, iSCSI Qualified Names (IQNs) need to be created.
These need to be unique.  So every target must have it's own unique
IQN, and every client (initiator) must have its own IQN.

After a pvc is being created, a corresponding target having specific IQN format `iqn.<YEAR>-<MM>.k8s.<namespace>:<pvc-name>` 
is created on iSCSI target server. 


## Install the iscsi provisioner pod in Kubernetes

This set of command will install iSCSI-targetd provisioner in the `default` namespace.
```
export NS=default
kubectl apply -f https://raw.githubusercontent.com/ogre0403/iscsi-provisioner/master/kubernetes/iscsi-provisioner-class.yaml -n $NS
kubectl apply -f https://raw.githubusercontent.com/ogre0403/iscsi-provisioner/master/kubernetes/iscsi-provisioner-d.yaml -n $NS
```

### Install the iscsi provisioner pod in Openshift

Run the following commands. The secret correspond to username and password you have chosen for targetd (admin is the default for the username)
```
oc new-project iscsi-provisioner
oc create sa iscsi-provisioner
oc adm policy add-cluster-role-to-user cluster-reader system:serviceaccount:iscsi-provisioner:iscsi-provisioner
# if Openshift is version < 3.6 add the iscsi-provisioner-runner role
oc create -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/openshift/iscsi-auth.yaml
# else if Openshift is version >= 3.6 add the system:persistent-volume-provisioner role
oc adm policy add-cluster-role-to-user system:persistent-volume-provisioner system:serviceaccount:iscsi-provisioner:iscsi-provisioner
#
oc secret new-basicauth targetd-account --username=admin --password=ciao
oc create -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/openshift/iscsi-provisioner-dc.yaml
```



### Create a storage class

storage classes should look like the following
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: iscsi-target-api-sc
provisioner: iscsi-target-api
parameters:
  targetPortal: 192.168.1.111:3260

```

### Test iscsi provisioner

Create a pvc
```
$ kubectl create -f https://raw.githubusercontent.com/ogre0403/iscsi-provisioner/master/kubernetes/iscsi-provisioner-pvc.yaml
```
verify that the pv & pvc has been created
```
$ kubectl get pvc
$ kubectl get pv
```
you may also want to verify that the volume has been created
```
$ tgt-admin -s
```
deploy a pod that uses the pvc
```
$ kubectl create -f https://raw.githubusercontent.com/ogre0403/iscsi-provisioner/master/kubernetes/iscsi-test-pod.yaml
```



## on iSCSI authentication

If you enable iSCSI CHAP-based authentication, the ansible installer will set the target configuration consinstently and also configure the storage class.
However at provisioning time the provisioner will not setup the chap secret. Having the permissions to setup a secret in any namespace would make the provisioner too powerful and insecure.
So, it is up to the project administrator to setup the secret.
The name of the expected secret name will be `<provisioner-name>-chap-secret` 
An example of the secret format can be found [here](./openshift/iscsi-chap-secret.yaml)

